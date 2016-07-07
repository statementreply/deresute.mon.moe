all: prep rankserver
	./rankserver
fetch: datafetcher
	./datafetcher

all: rankserver dumpbody datafetcher

prep:
	if [ ! -d "data" ]; then \
	    ln -s ../deresuteme/data; \
	fi

dumpbody: src/apiclient/apiclient.go
datafetcher: src/apiclient/apiclient.go
test1: src/apiclient/apiclient.go

%: %.go
	source ./go_env.sh; \
		go build $<
