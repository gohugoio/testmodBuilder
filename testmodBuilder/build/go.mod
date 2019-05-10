module github.com/gohugoio/hugoTestHelpers/testmodBuilder/build

go 1.12

require (
	github.com/alexflint/go-arg v1.0.0
	github.com/gohugoio/hugo v0.55.5
	github.com/gohugoio/hugoTestHelpers/testmodBuilder/mods v0.0.0-20190513081324-4ece7d32a289
	github.com/pkg/errors v0.8.1
	github.com/shurcooL/go v0.0.0-20190330031554-6713ea532688
	github.com/spf13/afero v1.2.2
)

replace github.com/gohugoio/hugo => /Users/bep/dev/go/gohugoio/hugo

replace github.com/gohugoio/hugoTestHelpers/testmodBuilder/mods => /Users/bep/dev/go/gohugoio/hugoTestHelpers/testmodBuilder/mods
