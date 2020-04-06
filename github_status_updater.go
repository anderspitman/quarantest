package main

import (
        "bytes"
        "fmt"
        "net/http"
        "io/ioutil"
        "encoding/json"
)

type GithubStatusUpdater struct {
        Owner string
        RepoName string
        Sha string
        IssueNumber int
        username string
        token string
        httpClient *http.Client
}

type GithubCredentials struct {
        Username string `json:"username"`
        Token string `json:"token"`
}

type GithubComment struct {
        Body string `json:"body"`
}

type GithubStatus struct {
        State string `json:"state"`
        TargetUrl string `json:"target_url"`
        Description string `json:"description"`
        Context string `json:"context"`
}


func NewGithubStatusUpdater() *GithubStatusUpdater {

        githubCredsFile, err := ioutil.ReadFile("github_credentials.json")
        if err != nil {
                panic(err)
        }

        githubCreds := &GithubCredentials{}
        err = json.Unmarshal(githubCredsFile, &githubCreds)
        if err != nil {
                panic(err)
        }

        return &GithubStatusUpdater {
                username: githubCreds.Username,
                token: githubCreds.Token,
                httpClient: &http.Client{},
        }
}

func (c *GithubStatusUpdater) AddComment(comment *GithubComment) error {

        jsonStr, err := json.Marshal(comment)
        if err != nil {
                return err
        }

        endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments", c.Owner, c.RepoName, c.IssueNumber)

        fmt.Println(endpoint)

        req, err := http.NewRequest("POST", endpoint, bytes.NewReader(jsonStr))
        if err != nil {
                return err
        }

        req.Header.Set("Content-Type", "application/json; charset=utf-8")
        req.SetBasicAuth(c.username, c.token)

        _, err = c.httpClient.Do(req)
        if err != nil {
                fmt.Println(err)
                return err
        }

        return nil

}

func (c *GithubStatusUpdater) SetStatus(status *GithubStatus) error {

        jsonStr, err := json.Marshal(status)
        if err != nil {
                return err
        }

        endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/statuses/%s", c.Owner, c.RepoName, c.Sha)
        req, err := http.NewRequest("POST", endpoint, bytes.NewReader(jsonStr))
        if err != nil {
                return err
        }

        req.Header.Set("Content-Type", "application/json; charset=utf-8")
        req.SetBasicAuth(c.username, c.token)

        _, err = c.httpClient.Do(req)
        if err != nil {
                return err
        }

        return nil
}
