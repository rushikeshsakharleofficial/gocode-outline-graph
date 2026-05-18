package parser_test

import (
	"testing"

	"gocode-outline-graph/internal/parser"
)

func TestIsSupported_acceptsImplementedLanguages(t *testing.T) {
	cases := []string{
		"main.py", "index.js", "app.mjs", "mod.cjs",
		"types.ts", "App.tsx", "main.go", "lib.rs",
		"Hello.java", "lib.c", "util.h",
		"src.cpp", "src.cc", "src.cxx", "src.hpp", "src.hh", "src.hxx",
		"script.rb", "run.sh", "run.bash",
	}
	for _, f := range cases {
		if !parser.IsSupported(f) {
			t.Errorf("expected %q to be supported", f)
		}
	}
}

func TestIsSupported_rejectsBrokenExtensions(t *testing.T) {
	cases := []string{
		"data.json", "config.yaml", "config.yml",
		"page.html", "page.htm", "style.css",
		"notes.md", "notes.markdown",
		"query.sql", "App.vue", "App.svelte",
		"lib.lua", "Prog.cs", "Main.kt", "Main.kts",
		"App.swift", "lib.php", "analysis.r", "analysis.R",
		"lib.dart", "main.zig", "lib.toml", "Main.scala",
	}
	for _, f := range cases {
		if parser.IsSupported(f) {
			t.Errorf("expected %q to NOT be supported", f)
		}
	}
}
