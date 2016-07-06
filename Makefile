
all: prep websimple
	./websimple
run: build
	./main2

prep:
	if [ ! -d "data" ]; then \
	    ln -s ../deresuteme/data; \
	fi
build: dumpbody main2

dumpbody: src/apiclient/apiclient.go
main2: src/apiclient/apiclient.go

%: %.go
	source ./go_env.sh; \
		go build $<
