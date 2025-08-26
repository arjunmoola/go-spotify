dir := $(HOME)/.go-spotify/go-spotify.db

build:
	go build -o bin/gsp go-spotify/cmd/gsp 

install:
	go install go-spotify/cmd/gsp

clean:
	rm $(dir)
