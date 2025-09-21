.PHONY = install build clean

dir := $(HOME)/.go-spotify/go-spotify.db

install:
	go install github.com/arjunmoola/go-spotify/cmd/gsp

build:
	go build -o bin/gsp github.com/arjunmoola/go-spotify/cmd/gsp 

clean:
	rm $(dir)
