# P4dbeat

Welcome to P4dbeat.

Ensure that this folder is at the following location:
`${GOPATH}/src/github.com/rcowham/p4dbeat`

## Getting Started with P4dbeat

### Requirements

* [Golang](https://golang.org/dl/) 1.14

### Init Project
To get running with P4dbeat and also install the
dependencies, run the following command:

```
make setup
```

It will create a clean git history for each major step. Note that you can always rewrite the history if you wish before pushing your changes.

To push P4dbeat in the git repository, run the following commands:

```
git remote set-url origin https://github.com/rcowham/p4dbeat
git push origin master
```

For further development, check out the [beat developer guide](https://www.elastic.co/guide/en/beats/libbeat/current/new-beat.html).

### Build

To build the binary for P4dbeat run the command below. This will generate a binary
in the same directory with the name p4dbeat.

```
make ES_BEATS=$GOPATH/pkg/mod/github.com/elastic/beats/v7@v7.9.1 p4dbeat
```


### Run

To run P4dbeat with debugging output enabled, run:

```
./p4dbeat -c p4dbeat.yml -e -d "*"
```


### Test

To test P4dbeat, run the following command:

```
make testsuite
```

alternatively:
```
make unit-tests
make system-tests
make integration-tests
make coverage-report
```

The test coverage is reported in the folder `./build/coverage/`

### Update

Each beat has a template for the mapping in elasticsearch and a documentation for the fields
which is automatically generated based on `fields.yml` by running the following command.

```
make update
```


### Cleanup

To clean  P4dbeat source code, run the following commands:

```
make fmt
make simplify
```

To clean up the build directory and generated artifacts, run:

```
make clean
```


### Clone

To clone P4dbeat from the git repository, run the following commands:

```
mkdir -p ${GOPATH}/src/github.com/rcowham/p4dbeat
git clone https://github.com/rcowham/p4dbeat ${GOPATH}/src/github.com/rcowham/p4dbeat
```


For further development, check out the [beat developer guide](https://www.elastic.co/guide/en/beats/libbeat/current/new-beat.html).


## Packaging

The beat frameworks provides tools to crosscompile and package your beat for different platforms. This requires [docker](https://www.docker.com/) and vendoring as described above. To build packages of your beat, run the following command:

```
make package
```

This will fetch and create all images required for the build process. The whole process to finish can take several minutes.
