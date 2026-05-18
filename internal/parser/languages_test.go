package parser_test

import (
	"testing"

	"gocode-outline-graph/internal/parser"
)

func TestIsSupported_acceptsImplementedLanguages(t *testing.T) {
	cases := []string{
		// existing
		"main.py", "index.js", "app.mjs", "mod.cjs",
		"types.ts", "App.tsx", "main.go", "lib.rs",
		"Hello.java", "lib.c", "util.h",
		"src.cpp", "src.cc", "src.cxx", "src.hpp", "src.hh", "src.hxx",
		"script.rb", "run.sh", "run.bash",
		// new tree-sitter
		"App.cs", "index.php", "App.swift", "Main.kt", "Main.kts", "Main.scala",
		"config.yaml", "config.yml", "config.toml", "main.tf", "infra.hcl",
		"api.proto", "data.json",
		"page.html", "page.htm", "style.css", "style.scss", "style.sass", "style.less",
		"App.svelte", "App.vue",
		"lib.lua", "module.ex", "config.exs", "build.groovy", "build.gradle",
		"lib.ml", "lib.mli",
		"README.md", "README.mdx", "schema.sql",
		// fallback languages
		"main.dart", "main.zig",
		"core.clj", "core.cljs", "core.cljc",
		"server.erl", "header.hrl",
		"Main.hs", "Lib.lhs",
		"config.nix", "config.fish",
		"script.pl", "Module.pm", "test.t",
		"analysis.r", "analysis.R",
		"deploy.ps1", "module.psm1",
		"build.bat", "run.cmd",
		"schema.graphql", "query.gql",
		"config.xml",
		"build.mk", "Makefile", "makefile",
		"Dockerfile", "Dockerfile.dev",
	}
	for _, f := range cases {
		if !parser.IsSupported(f) {
			t.Errorf("expected %q to be supported", f)
		}
	}
}

func TestIsSupported_rejectsUnknownExtensions(t *testing.T) {
	cases := []string{
		"binary.exe", "archive.zip", "image.png", "data.parquet",
		"noextension", ".hidden",
	}
	for _, f := range cases {
		if parser.IsSupported(f) {
			t.Errorf("expected %q to NOT be supported", f)
		}
	}
}
