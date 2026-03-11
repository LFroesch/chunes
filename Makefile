build:
	go build -o chunes
cp:
	cp chunes ~/.local/bin/
	
install: build cp