MAIN := rankserver dfticker
EXTRA := pcapdump dumpbody unlz4 res get_profile test1 ticker df
all: ${MAIN}
extra: ${EXTRA}
clean:
	rm -fv ${MAIN} ${EXTRA}

web: prep rankserver
	./rankserver
fetch: df
	./df


prep:
	if [ ! -d "data" ]; then \
	    ln -s ../deresuteme/data; \
	fi

dumpbody: src/apiclient/*
df: src/datafetcher/* src/apiclient/*
dfticker: src/datafetcher/* src/apiclient/*
test1: src/apiclient/*
pcapdump: src/apiclient/*
unlz4: src/resource_mgr/*
res: src/resource_mgr/*
rankserver: src/resource_mgr/*
get_profile: src/apiclient/*

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
