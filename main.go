package main

import (
        "fmt"
        "log"
        "flag"
        "mime"
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
        PullRequest *GithubPullRequest `json:"pull_request"`
        Repository *GithubRepository `json:"repository"`
        //HeadCommit *GithubCommit `json:"head_commit"`
}

type GithubRepository struct {
        HtmlUrl string `json:"html_url"`
        Name string `json:"name"`
        Owner *GithubUser `json:"owner"`
}

type GithubCommit struct {
        Id string `json:"id"`
}

type GithubUser struct {
        Login string `json:"login"`
}

type GithubPullRequest struct {
        Head *GithubHead `json:"head"`
        Number int `json:"number"`
}

type GithubHead struct {
        Sha string `json:"sha"`
        Repo *GithubRepository `json:"repo"`
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

                        files, err := ioutil.ReadDir(commitDir)
                        if err != nil {
                                fmt.Fprintln(os.Stderr, err)
                                w.WriteHeader(500)
                                return
                        }

                        var matches []string
                        for _, file := range files {
                                if strings.HasPrefix(file.Name(), sha) {
                                        matches = append(matches, file.Name())
                                }
                        }

                        if len(matches) < 1 {
                                fmt.Fprintln(os.Stderr, err)
                                w.WriteHeader(400)
                                w.Write([]byte(fmt.Sprintf("sha '%s' did not match any demos", sha)))
                                return
                        }

                        if len(matches) > 1 {
                                fmt.Fprintln(os.Stderr, err)
                                w.WriteHeader(400)
                                w.Write([]byte(fmt.Sprintf("sha '%s' is not specific enough. Try using the full sha", sha)))
                                return
                        }

                        fullSha := matches[0]


                        rootDir := path.Join(commitDir, fullSha, "build")

                        if _, err := os.Stat(rootDir); !os.IsNotExist(err) {
                                filePath := path.Join(rootDir, r.URL.Path)
                                log.Println(r.Method + " " + fullSha + r.URL.Path)

                                _, err := os.Stat(filePath)
                                if r.URL.Path == "/" || os.IsNotExist(err) {
                                        contentType := mime.TypeByExtension("index.html")
                                        w.Header().Set("Content-Type", contentType)
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

                                        contentType := mime.TypeByExtension(path.Ext(filePath))
                                        if contentType != "" {
                                                w.Header().Set("Content-Type", contentType)
                                        }

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
        statusUpdater.Owner = webhook.Repository.Owner.Login
        statusUpdater.RepoName = webhook.Repository.Name
        statusUpdater.Sha = webhook.PullRequest.Head.Sha
        statusUpdater.IssueNumber = webhook.PullRequest.Number

        shortSha := webhook.PullRequest.Head.Sha[:8]
        targetUrl := fmt.Sprintf("http://%s.quarantest.iobio.io", shortSha)

        //pendingStatus := &GithubStatus{
        //        State: "pending",
        //        TargetUrl: targetUrl,
        //        Description: "quarantest build started",
        //        Context: "testing/quarantest",
        //}

        //failureStatus := &GithubStatus{
        //        State: "failure",
        //        TargetUrl: targetUrl,
        //        Description: "quarantest build failed",
        //        Context: "testing/quarantest",
        //}

        //successStatus := &GithubStatus{
        //        State: "success",
        //        TargetUrl: targetUrl,
        //        Description: "quarantest build succeeded",
        //        Context: "testing/quarantest",
        //}

        //pendingComment := &GithubComment{
        //        Body: "quarantest build started",
        //}

        successComment := &GithubComment{
                Body: fmt.Sprintf("quarantest build successful. Demo link at\n%s", targetUrl),
        }

        //err := statusUpdater.SetStatus(pendingStatus)
        //err := statusUpdater.AddComment(pendingComment)

        srcDir := path.Join(commitDir, webhook.PullRequest.Head.Sha, "src")

        log.Println(webhook.PullRequest.Head.Sha, "clone repository")

        cloneCommand := exec.Command("git", "clone", webhook.PullRequest.Head.Repo.HtmlUrl, srcDir)
        _, err := cloneCommand.Output()
        if err != nil {
                //err = statusUpdater.SetStatus(failureStatus)
                fmt.Println(err)
                w.WriteHeader(400)
                fmt.Fprintf(w, "%s", err)
                return
        }

        log.Println(webhook.PullRequest.Head.Sha, "checkout version")

        args := []string{"-C", srcDir, "checkout", webhook.PullRequest.Head.Sha}
        checkoutCommand := exec.Command("git", args...)
        _, err = checkoutCommand.Output()
        if err != nil {
                //err = statusUpdater.SetStatus(failureStatus)
                fmt.Println(err.(*exec.ExitError).Stderr)
                w.WriteHeader(400)
                return
        }

        log.Println(webhook.PullRequest.Head.Sha, "read config")

        quarantestConfigFilePath := path.Join(srcDir, "quarantest.json")
        quarantestConfigFile, err := ioutil.ReadFile(quarantestConfigFilePath)
        if err != nil {
                //err = statusUpdater.SetStatus(failureStatus)
                w.WriteHeader(400)
                fmt.Fprintf(w, "%s", err)
                return
        }

        quarantestConfig := &QuarantestConfig{}
        err = json.Unmarshal(quarantestConfigFile, &quarantestConfig)
        if err != nil {
                //err = statusUpdater.SetStatus(failureStatus)
                w.WriteHeader(400)
                fmt.Fprintf(w, "%s", err)
                return
        }

        dockerImage := quarantestConfig.DockerImage
        buildScriptPath := "/src/" + quarantestConfig.BuildScript

        log.Println(webhook.PullRequest.Head.Sha, "create build directory")

        buildDir := path.Join(commitDir, webhook.PullRequest.Head.Sha, "build")
        err = os.MkdirAll(buildDir, os.ModeDir | 0755)
        if err != nil {
                //err = statusUpdater.SetStatus(failureStatus)
                fmt.Println(err.(*exec.ExitError).Stderr)
                w.WriteHeader(400)
                return
        }


        log.Println(webhook.PullRequest.Head.Sha, "run build")

        srcMount := fmt.Sprintf("type=bind,source=%s,target=/src", srcDir)
        buildMount := fmt.Sprintf("type=bind,source=%s,target=/build", buildDir)
        args = []string{"run", "--rm", "-i", "--mount", srcMount, "--mount", buildMount, dockerImage, buildScriptPath}
        buildCommand := exec.Command("docker", args...)
        //buildCommand.Dir = outDir
        _, err = buildCommand.Output()
        if err != nil {
                //err = statusUpdater.SetStatus(failureStatus)
                fmt.Println(err)
                fmt.Println(err.(*exec.ExitError).Stderr)
                w.WriteHeader(400)
                return
        }

        //err = statusUpdater.SetStatus(successStatus)
        err = statusUpdater.AddComment(successComment)

        log.Println(webhook.PullRequest.Head.Sha, "done")
}
