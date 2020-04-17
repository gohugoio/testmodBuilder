package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gohugoio/hugo/modules"
	testmods "github.com/gohugoio/testmodBuilder/mods"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

const (

	// Increment the minor version.
	versionTemplate = "v1.%d.0"
	testGitRepoBase = "hugoTestModules1"
)

func main() {
	// Run this on darwin, linux and windows.
	goos := runtime.GOOS
	gitRepo := testGitRepoBase + "_" + goos

	dir, err := ioutil.TempDir("", "hugotestmods")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	fs := afero.NewOsFs()
	m := testmods.CreateModules(goos)

	b := &mb{
		fs:      fs,
		mods:    m.Collect(),
		environ: os.Environ(),
	}

	b.cdir(dir)

	must(b.run("git", "clone", fmt.Sprintf("git@github.com:gohugoio/%s.git", gitRepo)))
	b.cdir(filepath.Join(dir, gitRepo))
	must(b.all())

}

type mb struct {

	// Increments from cfg.Version
	currentMinorVersion int

	fs      afero.Fs
	mods    []*testmods.Md
	dir     string
	environ []string
}

func (b *mb) cdir(dir string) {
	b.environ = append(b.environ, "PWD="+dir)
	b.dir = dir
}

func (b *mb) createFiles() error {
	if err := b.mkDirs(); err != nil {
		return err
	}

	if err := b.mkConfigs(); err != nil {
		return err
	}

	if err := b.mkDataFiles(); err != nil {
		return err
	}

	return nil
}

func (b *mb) initGoMods() error {
	b.running("initGoMods")
	for _, m := range b.mods {
		hm := b.newModulesHandler(m)
		if err := hm.Init(m.Path()); err != nil {
			return err
		}
	}

	return nil
}

func (b *mb) newModulesHandler(m *testmods.Md) *modules.Client {

	pths := m.Paths()
	imports := make([]modules.Import, len(pths))
	for i, v := range pths {
		imports[i] = modules.Import{Path: v}
	}

	modConfig := modules.DefaultModuleConfig
	modConfig.Imports = imports

	client := modules.NewClient(modules.ClientConfig{
		Fs:           b.fs,
		WorkingDir:   b.abs(m.Name()),
		IgnoreVendor: true,
		ModuleConfig: modConfig,
	})

	return client
}

func (b *mb) abs(name string) string {
	return filepath.Join(b.dir, name)
}

func (b *mb) clean() error {
	b.running("clean")

	// The clean script will be run with the target repo's PWD, so
	// we need to make the path absolute.
	dir, _ := os.Getwd()
	if err := b.run("bash", filepath.Join(dir, "clean.sh")); err != nil {
		return err
	}

	// Clean the relevant part of the mod cache.
	gp := os.Getenv("GOPATH")
	if gp == "" {
		return errors.New("GOPATH not set, cannot clean")
	}

	modDir := filepath.Join(gp, "pkg/mod")

	// Walk the directory and remove any directory with a name matching one
	// of our modules.
	err := afero.Walk(b.fs, modDir, func(path string, info os.FileInfo, err error) error {
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if info.IsDir() {
			for _, m := range b.mods {
				if strings.Contains(path, m.Name()) {
					b.removeModDir(path)
					break
				}
			}
		}
		return nil
	})

	return err

}

func (b *mb) collectModules() error {
	b.running("collectModules")
	for _, m := range b.mods {
		hm := b.newModulesHandler(m)
		if _, err := hm.Collect(); err != nil {
			return err
		}
	}
	return nil

}

func (b *mb) commit(msg string, force bool) error {
	b.running("commit")
	must(b.run("git", "add", "."))
	err := b.run("git", "commit", "-m", fmt.Sprintf("[%s] %s", b.version(), msg))
	if err != nil {
		log.Println("warning: commit: ", err)
	}

	if force {
		return b.pushForce()
	} else {
		return b.push()
	}
}

func (b *mb) mkConfigs() error {
	b.running("mkconfigs")
	for _, m := range b.mods {
		config := fmt.Sprintf(`
theme = %s
`, m.PathsStr())

		if err := afero.WriteFile(b.fs, b.abs(filepath.Join(m.Name(), "config.toml")), []byte(config), 0666); err != nil {
			return err
		}
	}

	return nil
}

func (b *mb) mkDirs() error {
	b.running("mkdirs")
	for _, m := range b.mods {
		if err := b.fs.Mkdir(b.abs(m.Name()), 0777); err != nil {
			return err
		}
	}
	return nil
}

func (b *mb) mkDataFiles() error {
	fileContentPairs := b.dataFiles()

	for _, m := range b.mods {
		dataDir := b.abs(filepath.Join(m.Name(), "data"))
		if err := b.fs.RemoveAll(dataDir); err != nil {
			return err
		}
		if err := b.fs.MkdirAll(filepath.Join(dataDir, "modinfo"), 0777); err != nil {
			return err
		}
	}

	for i := 0; i < len(fileContentPairs); i += 2 {
		path, content := fileContentPairs[i], fileContentPairs[i+1]
		if err := afero.WriteFile(b.fs, b.abs(path), []byte(content), 0666); err != nil {
			return err
		}
	}

	return nil
}

func (b *mb) nextVersion() {
	b.currentMinorVersion++
}

func (b *mb) push() error {
	return b.run("git", "push")
}

func (b *mb) pushForce() error {
	b.running("pushForce")
	return b.run("git", "push", "-f", "--tags")
}

func (b *mb) pushTags() error {
	b.running("pushTags")
	return b.run("git", "push", "--tags")
}

func (b *mb) removeModDir(dir string) error {
	// Module cache has 0555 directories; make them writable in order to remove content.
	afero.Walk(b.fs, dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			os.Chmod(path, 0777)
		}
		return nil
	})
	return b.fs.RemoveAll(dir)

}

func (b *mb) run(bin string, args ...string) error {

	stderr := new(bytes.Buffer)
	cmd := exec.Command(bin, args...)

	cmd.Env = b.environ
	cmd.Dir = b.dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.MultiWriter(stderr, os.Stderr)

	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.Error); ok && ee.Err == exec.ErrNotFound {
			return errors.Errorf("%s not found", bin)
		}

		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			return errors.Errorf("failed to execute '%s %v': %s %T", bin, args, err, err)
		}

		return errors.Errorf("%s command failed: %s: %s", bin, exitErr, stderr)

	}

	return nil
}

func (b *mb) running(name string) {
	fmt.Printf("\nRunning %s…\n\n", name)
}

// Return path/content pairs for the data files.
func (b *mb) dataFiles() []string {
	// module.toml
	// name.toml
	var sf []string

	version := b.version()

	for _, m := range b.mods {
		name := m.Name()
		content := fmt.Sprintf(`name = %q
version = %q
`, name, version)
		sf = append(sf, filepath.Join(name, "data/modinfo", "module.toml"), content)
		sf = append(sf, filepath.Join(name, "data/modinfo", name+".toml"), content)
	}

	return sf
}

func (b *mb) tagAndUpdateMods() error {
	if err := b.tagVersions(); err != nil {
		return err
	}

	if err := b.updateModules(); err != nil {
		return err
	}

	if err := b.commit("Update modules", false); err != nil {
		return err
	}

	b.nextVersion()

	return nil
}

func (b *mb) tagVersions() error {
	b.running("tagVersions")
	version := fmt.Sprintf(versionTemplate, b.currentMinorVersion)
	for _, m := range b.mods {
		tag := path.Join(m.Name(), version)
		if err := b.run("git", "tag", "-a", tag, "-m", fmt.Sprintf("version %s of %s", version, m.Name())); err != nil {
			return err
		}
	}

	if err := b.pushTags(); err != nil {
		return err
	}

	return b.run("git", "--no-pager", "tag", "-l")
}

func (b *mb) tidyModules() error {
	b.running("tidyModules")
	for _, m := range b.mods {
		hm := b.newModulesHandler(m)
		if err := hm.Tidy(); err != nil {
			return err
		}
	}
	return nil
}

func (b *mb) updateModules() error {
	b.running("updateModules")
	for _, m := range b.mods {
		hm := b.newModulesHandler(m)
		if err := hm.Get("-u"); err != nil {
			return err
		}
	}
	return nil
}

func (b *mb) vendorModules() error {
	b.running("vendorModules")
	var vendored bool
	for _, m := range b.mods {
		if !m.Vendor {
			continue
		}
		vendored = true
		hm := b.newModulesHandler(m)
		if err := hm.Vendor(); err != nil {
			return err
		}
	}

	if vendored {
		must(b.commit("Vendor modules", false))
	}

	return nil

}

func (b *mb) version() string {
	return b.versionFor(b.currentMinorVersion)
}

func (b *mb) versionFor(minorVersion int) string {
	return fmt.Sprintf(versionTemplate, minorVersion)
}

// Start fresh and build all test modules.
// TODO(bep) set up a travis build that builds from Linux, OSX and Windows and
// use all of those in the tests.
func (b *mb) all() error {
	must(b.allToVendor())

	must(b.mkDataFiles())
	must(b.commit("Add new set of static files", false))
	must(b.tagAndUpdateMods())

	must(b.tidyModules())
	must(b.commit("Tidy modules", false))
	must(b.mkDataFiles())
	must(b.commit("Add new set of static files", false))
	must(b.tagAndUpdateMods())

	return nil
}

func (b *mb) allToVendor() error {
	must(b.clean())

	must(b.createFiles())
	must(b.initGoMods())
	must(b.commit("Add initial version of modules", false))

	must(b.collectModules())
	must(b.commit("Collect modules from Hugo config", false))

	must(b.tagAndUpdateMods())

	must(b.mkDataFiles())
	must(b.commit("Add new set of static files", false))

	must(b.tagAndUpdateMods())

	must(b.vendorModules())
	must(b.tagAndUpdateMods())

	return nil
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
