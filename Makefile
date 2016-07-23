MAIN := rankserver dfnew ticker
EXTRA := pcapdump dumpbody unlz4 res get_profile test1 datafetcher
all: ${MAIN}
extra: ${EXTRA}
clean:
	rm -fv ${MAIN} ${EXTRA}

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
dfnew: src/datafetcher/*
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
