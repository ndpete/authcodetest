# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
OUTPUTFOLDER=./dist
BINARY_NAME=authcodetest

all: test build
build-all: build-linux build-mac build-win 
clean: 
		$(GOCLEAN)
		rm -rf ./dist


# Cross compilation
build-linux:
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(OUTPUTFOLDER)/LINUX/$(BINARY_NAME) -v && tar czvf $(OUTPUTFOLDER)/$(BINARY_NAME)-LINUX.tgz -C $(OUTPUTFOLDER)/LINUX/ .

build-mac:
		CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(OUTPUTFOLDER)/MAC/$(BINARY_NAME) -v && zip -j $(OUTPUTFOLDER)/$(BINARY_NAME)-MAC.zip $(OUTPUTFOLDER)/MAC/$(BINARY_NAME)

build-win:
		CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(OUTPUTFOLDER)/WIN/$(BINARY_NAME) -v && zip -j $(OUTPUTFOLDER)/$(BINARY_NAME)-win.zip $(OUTPUTFOLDER)/WIN/$(BINARY_NAME)