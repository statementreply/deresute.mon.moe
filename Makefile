
all: prep rankserver
	./rankserver
run: build
	./datafetcher

prep:
	if [ ! -d "data" ]; then \
	    ln -s ../deresuteme/data; \
	fi
build: dumpbody datafetcher

dumpbody: src/apiclient/apiclient.go
datafetcher: src/apiclient/apiclient.go

%: %.go
	source ./go_env.sh; \
		go build $<
