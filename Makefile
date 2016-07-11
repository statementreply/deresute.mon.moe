all: rankserver dumpbody datafetcher pcapdump

web: prep rankserver
	./rankserver
fetch: datafetcher
	./datafetcher


prep:
	if [ ! -d "data" ]; then \
	    ln -s ../deresuteme/data; \
	fi

dumpbody: src/apiclient/apiclient.go
datafetcher: src/apiclient/apiclient.go
test1: src/apiclient/apiclient.go

%: %.go
	source ./go_env.sh; \
		go build -x -i $<

precompile:
	source ./go_env.sh; \
	go install apiclient; \
	go install gopkg.in/yaml.v2

linksys:
	ln -s /usr/share/gocode/src/github.com src/ || true
	ln -s /usr/share/gocode/src/gopkg.in   src/ || true
