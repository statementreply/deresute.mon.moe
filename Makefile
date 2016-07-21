all: rankserver datafetcher ticker
extra: pcapdump dumpbody unlz4 res get_profile test1
clean:
	rm -fv	rankserver dumpbody datafetcher pcapdump

web: prep rankserver
	./rankserver
fetch: datafetcher
	./datafetcher


prep:
	if [ ! -d "data" ]; then \
	    ln -s ../deresuteme/data; \
	fi

dumpbody: src/apiclient/*
datafetcher: src/apiclient/*
test1: src/apiclient/*
pcapdump: src/apiclient/*
unlz4: src/resource_mgr/*
res: src/resource_mgr/*
rankserver: src/resource_mgr/*

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
