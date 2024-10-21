.PHONY: build clean install uninstall

APP_NAME=goback

default:
	@cat Makefile | tail -n +8  # strip content until actual commands

build:
	@go build
clean:
	@rm -f $(APP_NAME)
install:
	make clean
	go build
	rm -f ~/.local/bin/$(APP_NAME)
	cp $(APP_NAME) ~/.local/bin/$(APP_NAME)
uninstall:
	rm -f ~/.local/bin/$(APP_NAME)
