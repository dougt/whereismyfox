This is the server and web interface for the Where Is My Fox? application.

The server is written in [Go](http://golang.org).

Install:

    # set a valid GOPATH first. See http://golang.org/doc/code.html
    cd $GOPATH
    go get github.com/dougt/whereismyfox
    go install github.com/dougt/whereismyfox

Configure:

    # Copy the example config
    cd $GOPATH
    mkdir conf
    cp src/github.com/dougt/whereismyfox/config-example.json conf/whereismyfox.json
    # edit conf/whereismyfox.json

Run:

    cd $GOPATH
    ./bin/whereismyfox -config conf/whereismyfox.json -db whereismyfox.sqlite

To contribute, fork and send a pull request.
