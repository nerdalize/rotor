# Rotor Hello world
This example shows how rotor allows for deploying a DynamoDB powered API in under 15 minutes.

##  Dependencies
To get this example to work you'll need to following installed an available in your `$PATH`:

- [Go SDK >= 1.7](https://golang.org/dl/): for using the new `context.Context` interface
- [Glide](https://github.com/Masterminds/glide): for managing the necessary Go dependencies
- [Docker](https://github.com/docker/docker): for building a efficient Lambda packages from your Go code
- [Terraform](https://www.terraform.io): for managing the various infrastructure components

## Credentials
Before deploying it is necessary to have some valid AWS credentials available in the `./secrets.env` file. The file should look like this:

```
AWS_ACCESS_KEY_ID=<YOUR_ACCESS_KEY_ID>
AWS_SECRET_ACCESS_KEY=<YOUR_SECRET_ACCESS_KEY>
```

## Deploying the Application
The `make.sh` provide some convenient steps for getting the example online:

  1. Installing dependencies: `make.sh install`
  2. Building the Lambda package: `make.sh build`
  3. Deploy the application: `make.sh deploy`

Terraform should deploy your application and show a https endpoint that you can curl immediately. It will return the content of your empty DynamoDB table:

```
$ curl https://<YOUR_API_ID>.execute-api.eu-west-1.amazonaws.com/default
> []
```

## Integration Testing
Rotor is specifically setup to be easily testable. An example of a typical integration test can be found in `main_test.go`.

  1. Run the integration test: `make.sh test`

## Re-deploying Changes
To deploy changes you would need to rebuild the lambda package and redeploy it to AWS:

  1. Build a new package: `make.sh build`
  2. Deploy the new package: `make.sh deploy`

## Destroying the Deployment
If the application is no longer needed it recommended to the destroy the AWS resources:

  1. Destroy the infrastructure: `make.sh destroy`
