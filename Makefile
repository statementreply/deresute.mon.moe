MAIN := rankserver dfticker
EXTRA := pcapdump dumpbody unlz4 res get_profile test1 ticker df
all: ${MAIN}
extra: ${EXTRA}
clean:
	rm -fv ${MAIN} ${EXTRA}



server: rankserver
	./rankserver
fetch: dfticker
	./dfticker
twitter:
	perl periodic_twitter.pl cached_status https://deresuteborder.mon.moe/twitter 60
twitter2:
	perl periodic_twitter.pl cached_status_emblem https://deresuteborder.mon.moe/twitter_emblem 3600


prep:
	if [ ! -d "data" ]; then \
	    ln -s ../deresuteme/data; \
	fi

rankserver: src/resource_mgr/* src/apiclient/*
dfticker: src/datafetcher/* src/apiclient/* src/resource_mgr/*
dumpbody: src/apiclient/*
df: src/datafetcher/* src/apiclient/*
test1: src/apiclient/*
pcapdump: src/apiclient/*
unlz4: src/resource_mgr/*
res: src/resource_mgr/* src/apiclient/*
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
