package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

type input struct {
	Dir    string `json:"dir"`
	Ignore string `json:"ignore"`
	Output string `json:"output"`
}

type result struct {
	ZipBase64Sha256 string `json:"zip_base64sha256"`
	ZipFilename     string `json:"zip_filename"`
}

func zipdir(dir string, w io.Writer, ignore []string) (err error) {
	zw := zip.NewWriter(w)
	err = filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("failed to determine path '%s' relative to '%s': %v", path, dir, err)
		}

		isdir := fi.Mode().IsDir()
		for _, p := range ignore {
			match, merr := filepath.Match(p, rel)
			if merr != nil {
				return fmt.Errorf("failed to match pattern '%s': %v", p, err)
			}

			if match {
				if isdir {
					return filepath.SkipDir
				}

				return nil
			}
		}

		if isdir {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file '%s': %v", rel, err)
		}

		defer f.Close()

		//we carefully set header values ourselves to make sure the zips sha
		//is as deterministic as possible but still ships over file metadata
		//aws lambd equires (i.e. execution bit)
		fh := &zip.FileHeader{}
		fh.Name = rel
		fh.SetMode(fi.Mode())

		zfw, err := zw.CreateHeader(fh)
		if err != nil {
			return fmt.Errorf("failed to create zip header: %v", err)
		}

		n, err := io.Copy(zfw, f)
		if err != nil {
			return fmt.Errorf("failed to write file to zip: %v", err)
		}

		if n != fi.Size() {
			return fmt.Errorf("unexpected nr of bytes written to tar, saw '%d' on-disk but only wrote '%d', is directory '%s' in use?", fi.Size(), n, dir)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk dir '%s': %v", dir, err)
	}

	//add index.js the nodejs wrapper
	fh := &zip.FileHeader{}
	fh.Name = "index.js"
	zfw, err := zw.CreateHeader(fh)
	if err != nil {
		return fmt.Errorf("failed to create header: %v", err)
	}

	_, err = zfw.Write(indexjs)
	if err != nil {
		return fmt.Errorf("failed to write nodejs wrapper: %v", err)
	}

	//finally close the zip
	if err = zw.Close(); err != nil {
		return fmt.Errorf("failed to write remaining data: %v", err)
	}

	return nil
}

func pkg(logs *log.Logger, in *input) (res *result, err error) {
	if in.Dir == "" {
		return nil, fmt.Errorf("input didnt specify a directory to build in")
	}

	binpath := filepath.Join(in.Dir, "main")
	defer os.Remove(binpath)

	gobuild := exec.CommandContext(context.Background(), "go", "build", fmt.Sprintf("-o=%s", binpath))
	gobuild.Stderr = os.Stderr
	gobuild.Dir = in.Dir
	gobuild.Env = os.Environ()
	gobuild.Env = append(gobuild.Env, "GOOS=linux")
	gobuild.Env = append(gobuild.Env, "GOARCH=amd64")

	err = gobuild.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to 'go build' package in '%s': %v", in.Dir, err)
	}

	zipf, err := ioutil.TempFile("", "tf-golambda_")
	if err != nil {
		return nil, fmt.Errorf("failed to temp zip file: %v", err)
	}

	hash := sha256.New()
	mw := io.MultiWriter(hash, zipf)

	err = zipdir(in.Dir, mw, filepath.SplitList(in.Ignore))
	if err != nil {
		return nil, fmt.Errorf("failed to zip directory: %v", err)
	}

	err = zipf.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close zip file: %v", err)
	}

	sha := hash.Sum(nil)
	if in.Output == "" {
		in.Output = filepath.Join(os.TempDir(), fmt.Sprintf("%x.zip", sha))
	}

	err = os.Rename(zipf.Name(), in.Output)
	if err != nil {
		return nil, fmt.Errorf("failed to move zip file to '%s': %v", in.Output, err)
	}

	return &result{
		ZipBase64Sha256: base64.StdEncoding.EncodeToString(sha),
		ZipFilename:     in.Output,
	}, nil
}

func run(logs *log.Logger, r io.Reader, w io.Writer) (err error) {
	dec := json.NewDecoder(r)
	enc := json.NewEncoder(w)

	in := &input{}
	err = dec.Decode(&in)
	if err != nil {
		return fmt.Errorf("failed to decode input: %v", err)
	}

	res, err := pkg(logs, in)
	if err != nil {
		return fmt.Errorf("failed to package: %v", err)
	}

	err = enc.Encode(res)
	if err != nil {
		return fmt.Errorf("failed to encode result: %v", err)
	}

	return nil

}

func main() {
	logs := log.New(os.Stderr, "tf-golambda/", log.Lshortfile)
	err := run(logs, os.Stdin, os.Stdout)
	if err != nil {
		logs.Fatal(err)
	}
}

var indexjs = []byte(`
const spawn = require('child_process').spawn;
const readline = require('readline');

const proc = spawn('./main');
const procStderr = readline.createInterface({ input: proc.stderr });
const procStdout = readline.createInterface({ input: proc.stdout });

procStderr.on('line', (line) => console.log(line));

proc.on('error', (err) => console.error('failed to start child process: %s', err))
proc.on('exit', (code) => console.error('child process exited unexpectedly with code: %s', code))

exports.handle = function(event, context, cb) {
	context.callbackWaitsForEmptyEventLoop = false
	procStdout.on('line', (line) => {
		try {
			var obj = JSON.parse(line)
	    cb(obj.error, obj.value)
	  } catch (e) {
	    cb(e)
	  }
	});

	proc.stdin.write(JSON.stringify({event: event, context: context}))
}
`)
