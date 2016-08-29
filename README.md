# gosuv
[![Build Status](https://travis-ci.org/codeskyblue/gosuv.svg)](https://travis-ci.org/codeskyblue/gosuv)

## Program not implement
**Not done yet.**

golang port of python-supervisor

Features

* [ ] Realtime log view
* [ ] Github webhook
* [ ] Web control page

## Requirements
Go version at least `1.5+`

## Installation
```sh
go get -v github.com/codeskyblue/gosuv
```

## Usage
```sh
$ gosuv status
NAME		STATUS
timetest	running
$ gosuv help
...
```

## Configuration
Default config file stored in directory `$HOME/.gosuv/`

## Design
### Get or Update program
`<GET|PUT> /api/programs/:name`

### Add new program
`POST /api/programs`

### Del program
`DELETE /api/programs/:name`

## State
Only 4 states. [ref](http://supervisord.org/subprocess.html#process-states)

![states](docs/states.png)

# Plugin Design (todo)
Current plugins:

- [tailf](https://github.com/codeskyblue/gosuv-tailf)

All command plugin will store in `$HOME/.gosuv/cmdplugin`, gosuv will treat this plugin as a subcommand.

for example:

	$HOME/.gosuv/cmdplugin/ --.
		|- showpid/
			|- run

There is a directory `showpid`

When run `gosuv showpid`, file `run` will be called.


## Use libs
* <https://github.com/ahmetalpbalkan/govvv>

## LICENSE
[MIT](LICENSE)
