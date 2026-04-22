run:
	go run ./cmd/knotty

runwasm:
	go run github.com/hajimehoshi/wasmserve@latest ./cmd/knotty

# site builds the exact tree deployed to GitHub Pages. Mirrors the CI job
# so `python3 -m http.server -d _site` can preview the Pages build locally.
site:
	mkdir -p _site
	GOOS=js GOARCH=wasm go build -o _site/knotty.wasm ./cmd/knotty
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" _site/wasm_exec.js
	cp web/index.html _site/index.html

clean:
	rm -rf _site knotty

