foo: prefoo
	echo foo

prefoo: preprefoo
	echo blahblah

preprefoo:
	echo prefoo

# Tests
test-init:
	cd test/examples && bake init --skip

# Useful stuff
babel:
	babel lib/ -d src/

watch:
	watchd lib/* test/* package.json -c 'bake test'

release: version push publish

version:
	standard-version -m "%s"

push:
	git push origin master --tags

publish:
	npm publish

test: babel
	bake test-init
	mocha -R spec

docs:
	mocha -R markdown >> readme.md

# tests
foo: prefoo prefoobar
	echo foo

foo2:
	echo foo2

prefoo:
	echo prefoo

foobar: prefoobar
	echo foobar

prefoobar:
	echo blahblah

all: foo foo2 foobar
