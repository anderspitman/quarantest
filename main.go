package main

import (
        "fmt"
        "log"
        "flag"
        "strings"
	"net/http"
        "io/ioutil"
        "encoding/json"
        "os"
        "os/exec"
        "path"
        //"bytes"
)

type GithubWebhook struct {
        Ref string `json:"ref"`
        Repository *GithubRepository `json:"repository"`
        HeadCommit *GithubCommit `json:"head_commit"`
}

type GithubRepository struct {
        Url string `json:"url"`
        Name string `json:"name"`
        Owner *GithubUser `json:"owner"`
}

type GithubCommit struct {
        Id string `json:"id"`
}

type GithubUser struct {
        Name string `json:"name"`
}

type QuarantestConfig struct {
        BuildScript string `json:"build_script"`
        DockerImage string `json:"docker_image"`
}


func main() {
        fmt.Println("Starting up")
        port := flag.String("port", "9001", "Port")
        flag.Parse()

        cwd, err := os.Getwd()
        if err != nil {
            fmt.Println(err)
        }

        commitDir := path.Join(cwd, "commits")


	handler := func(w http.ResponseWriter, r *http.Request) {

		pathStr := r.URL.Path

                if strings.HasPrefix(pathStr, "/webhook") {
                        body, err := ioutil.ReadAll(r.Body)
                        if err != nil {
                                fmt.Println(err)
                                w.WriteHeader(400)
                                fmt.Fprintf(w, "%s", err)
                                return
                        }

                        webhook := &GithubWebhook{}
                        err = json.Unmarshal(body, &webhook)
                        if err != nil {
                                fmt.Println(err)
                                w.WriteHeader(400)
                                fmt.Fprintf(w, "%s", err)
                                return
                        }

                        go doBuild(w, r, commitDir, webhook)
                } else {

                        hostParts := strings.Split(r.Host, ".")
                        sha := hostParts[0]

                        rootDir := path.Join(commitDir, sha, "build")
                        fmt.Println(rootDir)

                        if _, err := os.Stat(rootDir); !os.IsNotExist(err) {
                                filePath := path.Join(rootDir, r.URL.Path)
                                fmt.Println(r.URL.Path)
                                fmt.Println(filePath)

                                _, err := os.Stat(filePath)
                                if r.URL.Path == "/" || os.IsNotExist(err) {
                                        indexPath := path.Join(rootDir, "index.html")
                                        file, err := ioutil.ReadFile(indexPath)
                                        if err != nil {
                                                fmt.Println(err)
                                                fmt.Println(err.(*exec.ExitError).Stderr)
                                                w.WriteHeader(400)
                                                return
                                        }

                                        w.Write(file)
                                } else {
                                        file, err := ioutil.ReadFile(filePath)
                                        if err != nil {
                                                fmt.Println(err)
                                                fmt.Println(err.(*exec.ExitError).Stderr)
                                                w.WriteHeader(400)
                                                return
                                        }

                                        w.Write(file)
                                }
                        } else {
                                w.Write([]byte("does not exist"))
                        }
                }
        }

        err = http.ListenAndServe(":"+*port, http.HandlerFunc(handler))
	if err != nil {
		fmt.Println(err)
	}
}


func doBuild(w http.ResponseWriter, r *http.Request, commitDir string, webhook *GithubWebhook) {

        statusUpdater := NewGithubStatusUpdater()
        statusUpdater.Owner = webhook.Repository.Owner.Name
        statusUpdater.RepoName = webhook.Repository.Name
        statusUpdater.Sha = webhook.HeadCommit.Id

        targetUrl := fmt.Sprintf("http://%s.quarantest.iobio.io", webhook.HeadCommit.Id)

        pendingStatus := &GithubStatus{
                State: "pending",
                TargetUrl: targetUrl,
                Description: "quarantest build started",
                Context: "testing/quarantest",
        }

        failureStatus := &GithubStatus{
                State: "failure",
                TargetUrl: targetUrl,
                Description: "quarantest build failed",
                Context: "testing/quarantest",
        }

        successStatus := &GithubStatus{
                State: "success",
                TargetUrl: targetUrl,
                Description: "quarantest build succeeded",
                Context: "testing/quarantest",
        }

        err := statusUpdater.SetStatus(pendingStatus)

        srcDir := path.Join(commitDir, webhook.HeadCommit.Id, "src")

        log.Println(webhook.HeadCommit.Id, "clone repository")

        cloneCommand := exec.Command("git", "clone", webhook.Repository.Url, srcDir)
        _, err = cloneCommand.Output()
        if err != nil {
                err = statusUpdater.SetStatus(failureStatus)
                fmt.Println(err)
                w.WriteHeader(400)
                fmt.Fprintf(w, "%s", err)
                return
        }

        log.Println(webhook.HeadCommit.Id, "checkout version")

        args := []string{"-C", srcDir, "checkout", webhook.HeadCommit.Id}
        checkoutCommand := exec.Command("git", args...)
        _, err = checkoutCommand.Output()
        if err != nil {
                err = statusUpdater.SetStatus(failureStatus)
                fmt.Println(err.(*exec.ExitError).Stderr)
                w.WriteHeader(400)
                return
        }

        log.Println(webhook.HeadCommit.Id, "read config")

        quarantestConfigFilePath := path.Join(srcDir, "quarantest.json")
        quarantestConfigFile, err := ioutil.ReadFile(quarantestConfigFilePath)
        if err != nil {
                err = statusUpdater.SetStatus(failureStatus)
                w.WriteHeader(400)
                fmt.Fprintf(w, "%s", err)
                return
        }

        quarantestConfig := &QuarantestConfig{}
        err = json.Unmarshal(quarantestConfigFile, &quarantestConfig)
        if err != nil {
                err = statusUpdater.SetStatus(failureStatus)
                w.WriteHeader(400)
                fmt.Fprintf(w, "%s", err)
                return
        }

        dockerImage := quarantestConfig.DockerImage
        buildScriptPath := "/src/" + quarantestConfig.BuildScript

        log.Println(webhook.HeadCommit.Id, "create build directory")

        buildDir := path.Join(commitDir, webhook.HeadCommit.Id, "build")
        err = os.MkdirAll(buildDir, os.ModeDir | 0755)
        if err != nil {
                err = statusUpdater.SetStatus(failureStatus)
                fmt.Println(err.(*exec.ExitError).Stderr)
                w.WriteHeader(400)
                return
        }


        log.Println(webhook.HeadCommit.Id, "run build")

        srcMount := fmt.Sprintf("type=bind,source=%s,target=/src", srcDir)
        buildMount := fmt.Sprintf("type=bind,source=%s,target=/build", buildDir)
        args = []string{"run", "--rm", "-i", "--mount", srcMount, "--mount", buildMount, dockerImage, buildScriptPath}
        buildCommand := exec.Command("docker", args...)
        //buildCommand.Dir = outDir
        _, err = buildCommand.Output()
        if err != nil {
                err = statusUpdater.SetStatus(failureStatus)
                fmt.Println(err)
                fmt.Println(err.(*exec.ExitError).Stderr)
                w.WriteHeader(400)
                return
        }

        err = statusUpdater.SetStatus(successStatus)

        log.Println(webhook.HeadCommit.Id, "done")
}
