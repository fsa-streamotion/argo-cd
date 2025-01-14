package app

import (
	"fmt"
	"io/ioutil"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/errors"
	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
	"github.com/argoproj/argo-cd/util/json"
)

// this implements the "when" part of given/when/then
//
// none of the func implement error checks, and that is complete intended, you should check for errors
// using the Then()
type Actions struct {
	context      *Context
	lastOutput   string
	lastError    error
	ignoreErrors bool
}

func (a *Actions) IgnoreErrors() *Actions {
	a.ignoreErrors = true
	return a
}

func (a *Actions) PatchFile(file string, jsonPath string) *Actions {
	fixture.Patch(a.context.path+"/"+file, jsonPath)
	return a
}

func (a *Actions) DeleteFile(file string) *Actions {
	fixture.Delete(a.context.path + "/" + file)
	return a
}

func (a *Actions) AddFile(fileName, fileContents string) *Actions {
	fixture.AddFile(a.context.path+"/"+fileName, fileContents)
	return a
}

func (a *Actions) CreateFromFile(handler func(app *Application)) *Actions {
	app := &Application{
		ObjectMeta: v1.ObjectMeta{
			Name: a.context.name,
		},
		Spec: ApplicationSpec{
			Project: a.context.project,
			Source: ApplicationSource{
				RepoURL: fixture.RepoURL(a.context.repoURLType),
				Path:    a.context.path,
			},
			Destination: ApplicationDestination{
				Server:    a.context.destServer,
				Namespace: fixture.DeploymentNamespace(),
			},
		},
	}
	if a.context.env != "" {
		app.Spec.Source.Ksonnet = &ApplicationSourceKsonnet{
			Environment: a.context.env,
		}
	}
	if a.context.namePrefix != "" {
		app.Spec.Source.Kustomize = &ApplicationSourceKustomize{
			NamePrefix: a.context.namePrefix,
		}
	}
	if a.context.configManagementPlugin != "" {
		app.Spec.Source.Plugin = &ApplicationSourcePlugin{
			Name: a.context.configManagementPlugin,
		}
	}

	if len(a.context.jsonnetTLAS) > 0 || len(a.context.parameters) > 0 {
		logrus.Fatal("Application parameters or json tlas are not supported")
	}

	handler(app)
	data := json.MustMarshal(app)
	tmpFile, err := ioutil.TempFile("", "")
	errors.CheckError(err)
	_, err = tmpFile.Write(data)
	errors.CheckError(err)

	a.runCli("app", "create", "-f", tmpFile.Name())
	return a
}

func (a *Actions) Create() *Actions {

	args := []string{
		"app", "create", a.context.name,
		"--repo", fixture.RepoURL(a.context.repoURLType),
		"--path", a.context.path,
		"--dest-server", a.context.destServer,
		"--dest-namespace", fixture.DeploymentNamespace(),
	}

	if a.context.env != "" {
		args = append(args, "--env", a.context.env)
	}

	for _, parameter := range a.context.parameters {
		args = append(args, "--parameter", parameter)
	}

	args = append(args, "--project", a.context.project)

	for _, jsonnetTLAParameter := range a.context.jsonnetTLAS {
		args = append(args, "--jsonnet-tlas", jsonnetTLAParameter)
	}

	if a.context.namePrefix != "" {
		args = append(args, "--nameprefix", a.context.namePrefix)
	}

	if a.context.configManagementPlugin != "" {
		args = append(args, "--config-management-plugin", a.context.configManagementPlugin)
	}

	a.runCli(args...)

	return a
}

func (a *Actions) Declarative(filename string) *Actions {
	return a.DeclarativeWithCustomRepo(filename, fixture.RepoURL(a.context.repoURLType))
}

func (a *Actions) DeclarativeWithCustomRepo(filename string, repoURL string) *Actions {
	values := map[string]interface{}{
		"ArgoCDNamespace":     fixture.ArgoCDNamespace,
		"DeploymentNamespace": fixture.DeploymentNamespace(),
		"Name":                a.context.name,
		"Path":                a.context.path,
		"Project":             a.context.project,
		"RepoURL":             repoURL,
	}
	a.lastOutput, a.lastError = fixture.Declarative(filename, values)
	a.verifyAction()
	return a
}

func (a *Actions) PatchApp(patch string) *Actions {
	a.runCli("app", "patch", a.context.name, "--patch", patch)
	return a
}

func (a *Actions) Sync() *Actions {
	args := []string{"app", "sync", a.context.name, "--timeout", fmt.Sprintf("%v", a.context.timeout)}

	if a.context.async {
		args = append(args, "--async")
	}

	if a.context.prune {
		args = append(args, "--prune")
	}

	if a.context.resource != "" {
		args = append(args, "--resource", a.context.resource)
	}

	if a.context.localPath != "" {
		args = append(args, "--local", a.context.localPath)
	}

	a.runCli(args...)

	return a
}

func (a *Actions) TerminateOp() *Actions {
	a.runCli("app", "terminate-op", a.context.name)
	return a
}

func (a *Actions) Refresh(refreshType RefreshType) *Actions {

	flag := map[RefreshType]string{
		RefreshTypeNormal: "--refresh",
		RefreshTypeHard:   "--hard-refresh",
	}[refreshType]

	a.runCli("app", "get", a.context.name, flag)

	return a
}

func (a *Actions) Delete(cascade bool) *Actions {
	a.runCli("app", "delete", a.context.name, fmt.Sprintf("--cascade=%v", cascade))
	return a
}

func (a *Actions) And(block func()) *Actions {
	block()
	return a
}

func (a *Actions) Then() *Consequences {
	return &Consequences{a.context, a}
}

func (a *Actions) runCli(args ...string) {
	a.lastOutput, a.lastError = fixture.RunCli(args...)
	a.verifyAction()
}

func (a *Actions) verifyAction() {
	if !a.ignoreErrors {
		a.Then().Expect(Success(""))
	}
}
