package main

import (
	"archive/zip"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	log.Printf("rotorgen %s", strings.Join(os.Args[1:], " "))
	if len(os.Args) < 1 {
		log.Fatalf("usage: rotorgen zip_path -- [extra_build_args]")
	}

	//everything after the double dash is passed to the build command as extra args
	hasExtraArgs := false
	appendArgs := []string{}
	for _, a := range os.Args[1:] {
		if a == "--" {
			hasExtraArgs = true
			continue
		}

		if hasExtraArgs {
			appendArgs = append(appendArgs, a)
		}
	}

	zfilePath, err := filepath.Abs(os.Args[1])
	if err != nil {
		log.Fatalf("Failed to determine absolute path to output zip '%s': %v", os.Args[1], err)
	}

	goFilePath := os.Getenv("GOFILE")
	if goFilePath == "" {
		log.Fatalf("No file provided, run with 'go generate' and a package that has a go:generate entry")
	}

	appendArgs = append(appendArgs, goFilePath)
	log.Printf("Generating lambda package for '%s', appended build args: %v", goFilePath, appendArgs)
	dir, err := ioutil.TempDir("", "rotorgen")
	if err != nil {
		log.Fatalf("Failed to create temporary working directory: %v", err)
	}

	//setup output path
	outPath := filepath.Join(dir, "main")
	log.Printf("Compile executable to '%s'", outPath)

	//building lambda binary to working directory
	args := []string{"build", "-o", outPath}
	args = append(args, appendArgs...)
	build := exec.Command("go", args...)
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	build.Env = append(build.Env, []string{
		"GOOS=linux",
		"GOARCH=amd64",
		"GOPATH=" + os.Getenv("GOPATH"),
		"GOROOT=" + os.Getenv("GOROOT")}...,
	)

	log.Printf("Running build 'go %s', environment: %v", strings.Join(args, " "), build.Env)
	err = build.Run()
	if err != nil {
		log.Fatalf("Failed to run go build command : %v", err)
	}

	//zip all files in the working directory to the requested output zip
	zipf, err := os.Create(zfilePath)
	if err != nil {
		log.Fatalf("Failed to create zip file: %v", err)
	}

	// Create a new zip archive.
	w := zip.NewWriter(zipf)

	//copy the go binary bytes
	mainf, err := os.Open(outPath)
	if err != nil {
		log.Fatalf("Failed open compiled main file '%s': %v", outPath, err)
	}

	fi, err := mainf.Stat()
	if err != nil {
		log.Fatalf("Failed to stat compiled file: %v", err)
	}

	fh, err := zip.FileInfoHeader(fi)
	if err != nil {
		log.Fatalf("Failed to create zip header: %v", err)
	}

	mainZipF, err := w.CreateHeader(fh)
	if err != nil {
		log.Fatalf("Failed to create zip entry for main file: %v", err)
	}

	defer mainf.Close()
	_, err = io.Copy(mainZipF, mainf)
	if err != nil {
		log.Fatalf("Failed to write executable bytes to zip file: %v", err)
	}

	//add the other files
	for _, file := range files {
		f, err := w.Create(file.Name)
		if err != nil {
			log.Fatal(err)
		}
		_, err = f.Write([]byte(file.Body))
		if err != nil {
			log.Fatal(err)
		}
	}

	//close the zip archive
	err = w.Close()
	if err != nil {
		log.Fatalf("Failed to close zip file: %v", err)
	}
}

var files = []struct {
	Name, Body string
}{
	//Index file wraps the main go executable
	{"index.js", `
var child = require('child_process')
var byline = require('./byline')

var ctx
var proc = child.spawn('./main', { stdio: ['pipe', 'pipe', process.stderr] })

proc.on('error', function(err){
  console.error('error: %s', err)
  process.exit(1)
})

proc.on('exit', function(code){
  console.error('exit: %s', code)
  process.exit(1)
})

var out = byline(proc.stdout)
out.on('data', function(line){
  var msg = JSON.parse(line)
  ctx.done(msg.error, msg.value)
})

exports.handler = function(event, context, callback) {
  ctx = context
  proc.stdin.write(JSON.stringify({
    "event": event,
    "context": context
  })+'\n');
}
`},
	//ByLine file allows JSON lines to be read one at a time
	{"byline.js", `
// Copyright (C) 2011-2015 John Hewson
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to
// deal in the Software without restriction, including without limitation the
// rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
// sell copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
// IN THE SOFTWARE.

var stream = require('stream'),
    util = require('util');

// convinience API
module.exports = function(readStream, options) {
  return module.exports.createStream(readStream, options);
};

// basic API
module.exports.createStream = function(readStream, options) {
  if (readStream) {
    return createLineStream(readStream, options);
  } else {
    return new LineStream(options);
  }
};

// deprecated API
module.exports.createLineStream = function(readStream) {
  console.log('WARNING: byline#createLineStream is deprecated and will be removed soon');
  return createLineStream(readStream);
};

function createLineStream(readStream, options) {
  if (!readStream) {
    throw new Error('expected readStream');
  }
  if (!readStream.readable) {
    throw new Error('readStream must be readable');
  }
  var ls = new LineStream(options);
  readStream.pipe(ls);
  return ls;
}

//
// using the new node v0.10 "streams2" API
//

module.exports.LineStream = LineStream;

function LineStream(options) {
  stream.Transform.call(this, options);
  options = options || {};

  // use objectMode to stop the output from being buffered
  // which re-concatanates the lines, just without newlines.
  this._readableState.objectMode = true;
  this._lineBuffer = [];
  this._keepEmptyLines = options.keepEmptyLines || false;
  this._lastChunkEndedWithCR = false;

  // take the source's encoding if we don't have one
  this.on('pipe', function(src) {
    if (!this.encoding) {
      // but we can't do this for old-style streams
      if (src instanceof stream.Readable) {
        this.encoding = src._readableState.encoding;
      }
    }
  });
}
util.inherits(LineStream, stream.Transform);

LineStream.prototype._transform = function(chunk, encoding, done) {
  // decode binary chunks as UTF-8
  encoding = encoding || 'utf8';

  if (Buffer.isBuffer(chunk)) {
    if (encoding == 'buffer') {
      chunk = chunk.toString(); // utf8
      encoding = 'utf8';
    }
    else {
     chunk = chunk.toString(encoding);
    }
  }
  this._chunkEncoding = encoding;

  var lines = chunk.split(/\r\n|\r|\n/g);

  // don't split CRLF which spans chunks
  if (this._lastChunkEndedWithCR && chunk[0] == '\n') {
    lines.shift();
  }

  if (this._lineBuffer.length > 0) {
    this._lineBuffer[this._lineBuffer.length - 1] += lines[0];
    lines.shift();
  }

  this._lastChunkEndedWithCR = chunk[chunk.length - 1] == '\r';
  this._lineBuffer = this._lineBuffer.concat(lines);
  this._pushBuffer(encoding, 1, done);
};

LineStream.prototype._pushBuffer = function(encoding, keep, done) {
  // always buffer the last (possibly partial) line
  while (this._lineBuffer.length > keep) {
    var line = this._lineBuffer.shift();
    // skip empty lines
    if (this._keepEmptyLines || line.length > 0 ) {
      if (!this.push(this._reencode(line, encoding))) {
        // when the high-water mark is reached, defer pushes until the next tick
        var self = this;
        setImmediate(function() {
          self._pushBuffer(encoding, keep, done);
        });
        return;
      }
    }
  }
  done();
};

LineStream.prototype._flush = function(done) {
  this._pushBuffer(this._chunkEncoding, 0, done);
};

// see Readable::push
LineStream.prototype._reencode = function(line, chunkEncoding) {
  if (this.encoding && this.encoding != chunkEncoding) {
    return new Buffer(line, chunkEncoding).toString(this.encoding);
  }
  else if (this.encoding) {
    // this should be the most common case, i.e. we're using an encoded source stream
    return line;
  }
  else {
    return new Buffer(line, chunkEncoding);
  }
};
`},
}
