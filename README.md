# authcodetest
CLI to run authcode tests

## Quick Start
1. Get binary from [releases page](https://github.com/byu-oit/auth-code-test/releases) or build from source
2. Run `authcodetest generate` to generate a config file. You will need your Consumer Key, Consumer Secret, Callback URL
3. Run `authcodetest test` to run and authcode test. The application you used should be subscribed to the Echov2 API for best results.
4. Run `authcodetest help` or `authcodetest help COMMAND` for more information

## Build From Source
1. Have a Go 1.x installed
2. Clone repo
3. Run `go build -v`
4. Run the Binary or copy to somewhere on path `./authcodetest`
5. Alternatively build and install in one go: `go install`

## USAGE
```NAME:
   AuthCode Test - Test authcode flow and get tokens from authcode and refresh token

USAGE:
   main [global options] command [command options] [arguments...]

VERSION:
   0.1.0

COMMANDS:
     test, t           run authcode flow test
     generate, gen, g  Generate config file
     help, h           Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h     show help
   --version, -v  print the version```