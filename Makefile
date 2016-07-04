run: build
	./main2

build: dumpbody main2

dumpbody: src/apiclient/apiclient.go
main2: src/apiclient/apiclient.go

%: %.go
	source ./go_env.sh; \
		go build $<
