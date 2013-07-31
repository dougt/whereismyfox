Install dependencies:

	go get github.com/emicklei/go-restful
	go get github.com/mattn/go-sqlite3
	go get github.com/gorilla/sessions

Get the code

	go get github.com/dougt/whereismyfox

This repository contains the server code in the server/ directory,
and the app and main site in the app/ and static/ directories. This is
a nuisance because the go tool expects .go files to reside in the root
of the repository, and so a simple go build won't work to build the server.
Just rely on the build.sh script for the time being:

	cd $GOPATH/src/github.com/dougt/whereismyfox && scripts/build.sh
	./whereismyfox # run the server

In the future the server code should probably be split in a different
repository.
