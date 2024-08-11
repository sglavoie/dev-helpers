.PHONY: build clean install uninstall

APP_NAME=goback

default:
	@cat Makefile | tail -n +8  # strip content until actual commands

build:
	@go build
clean:
	@rm $(APP_NAME)
install:
	cp $(APP_NAME) ~/.local/bin/$(APP_NAME)
uninstall:
	rm ~/.local/bin/$(APP_NAME)
