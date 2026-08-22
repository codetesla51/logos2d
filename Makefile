PREFIX ?= /usr/local

# Native Wayland by default: Ebiten's X11 backend goes through XWayland,
# which has a known upstream GLFW bug where keys get stuck pressed on
# focus/state changes. `make x11` opts back into the old behavior.
TAGS = wayland

build:
	go build -tags $(TAGS) -o logos2d .

x11:
	go build -o logos2d .

run: build
	./logos2d demo/void_runner/main.lgs

demo: build
	./logos2d demo/void_runner/main.lgs -auto

install: build
	install -m755 logos2d $(PREFIX)/bin/logos2d

clean:
	rm -f logos2d

.PHONY: build x11 run demo install clean
