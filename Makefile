MAIN := rankserver dfticker twitter_ticker compress_db
EXTRA := pcapdump dumpbody unlz4 res get_profile test1 ticker df
all: ${MAIN}
extra: ${EXTRA}
clean:
	rm -fv ${MAIN} ${EXTRA}


fmt:
	go fmt
	go fmt apiclient resource_mgr datafetcher rankserver rijndael_wrapper timestamp

server: rankserver
	./rankserver

fetch: dfticker
	#./dfticker
	# protect against crash
	time ./dfticker; while sleep 120; do time ./dfticker; done

twitter: twitter_ticker
	./twitter_ticker


prep:
	if [ ! -d "data" ]; then \
	    ln -s ../deresuteme/data; \
	fi

MYLIB := src/rankserver/* src/resource_mgr/* \
  src/apiclient/* src/datafetcher/* \
  src/rijndael/* src/rijndael_wrapper/* \
  src/timestamp/* src/util/*

rankserver: $(MYLIB)
dfticker: $(MYLIB)
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

setcap:
	sudo setcap cap_net_raw,cap_net_admin=eip ./pcapdump
capture:
	./pcapdump -i eth0 -f 'tcp and port 80'

prevent_update:
	curl https://deresuteborder.mon.moe/twitter >cached_status
	curl https://deresuteborder.mon.moe/twitter_emblem >cached_status_emblem
	curl https://deresuteborder.mon.moe/twitter_trophy >cached_status_trophy


check_err:
	ag '(?<!\w)_(?!\w).*?:?=' -G 'go'
	#ag '(?<!\w)_(?!\w).*?:?=' *.go src/apiclient/*.go src/datafetcher/*.go src/resource_mgr/*.go src/rijndael_wrapper/*.go src/timestamp/* src/util/*
