macky - Simple MU* Non-Client
=====

macky is a MU* client for Unix systems written in Go and inspired somewhat by [suckless.org's](http://suckless.org) [ii](http://http://tools.suckless.org/ii/) IRC client.

Like ii, interacting with macky is accomplished through filesystem tools.  Commands are sent by piping data to an `in` FIFO file, and the output is written to an `out` file.  Using a given MU* is then as simple as leveraging `cat`, `tail`, `echo` and the like or as complex as whatever other tools you want to bring to bare.

It works quite well in acme, for example, using two `win` windows, one reading the `out` file with `tail -f` and the other sending commands with `cat >> in`.

The program expects a certain directory structure, beginning with a `connections` subdirectory.  In this directory, you create additional subdirectories for each separate server account you want to connect to.  These subdirectories are where connection configuration is held (in a `conf` file), and is where the `in` FIFO and `out` files are created by the program.

A sample connection configuration is provided.

## Usage

`./macky [connection-name]`

Program messages will be sent to stdout; all else is sent to the `out` file, including command echoes.

At present it requires one instance of the program for each connection; this is on the roadmap to be changed.

## Configuration

A connection's `conf` file is a JSON-formatted file with the following fields:

* Address - this is the server address to connect to
* Port - the port to use
* TLS - true/false whether to use secure comms (not yet implemented)
* Login - the login string to use when connected, see below
* User - the username to use
* Pass - the password for the username

### Login

The configuration "Login" field is to allow for variances in MU* log in commands, and has a simple formatting.  In the sample `conf` file, the example given is `"connect %u %p"`.

Any `%u` in the string is replaced with the value in the User field, and `%p` is replaced with the value in the Pass field.

## Control Messages

Special commands prefixed with `CTL_` can be written to the `in` file to control macky itself.  At present only one command is implemented: `CTL_QUIT`, which closes the current connection and cleans up leftover FIFOs.

## TODO

* Implement TLS support
* Support for multiple concurrent connections
* A modular "rules" system for doing magic with output