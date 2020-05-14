Most CI testing tools focus on automated tests, but sometimes the changes are
very visual and you just want to give your team a demo of your pull request to
play with. quarantest runs a build for each GitHub PR, generates a URL for the
build, then posts a comment on the PR with a link to the build. You can see an
example of it in action [here][0]. Still in a pretty hacky state. Probably would
be better to use the GH status API with a link that goes to a page listing all
the past builds from the PR instead of spamming comments, but it's getting the
job done.

# How to use it

There are 3 steps to using quarantest:

1. Setting up the server process
2. Setting up the project repo
3. Pointing GitHub webhooks at the server


# Setting up the server

quarantest is a single binary. Simply run it on a publicly addressible server.
You'll also need a wildcard domain registration set up, as quarantest using
domains based of commit hashes. Running behind an HTTPS proxy is recommended
but not required.

You'll also need a `github_credentials.json` file in the working directory
for quarantest, for posting comments. It looks like this:

```json
{
  "username": "anderspitman",
  "token": "****************************"
}
```

# Setting up the project repo

Put a `quarantest.json` file in the root of the GitHub
repo for a frontend app you want to test. The file looks like this:

```json
{
  "build_script": "quarantest_build.sh",
  "docker_image": "anderspitman/iobio-nodejs-12"
}
```

You can see that it references a build script and a Docker image. The build
script is what quarantest uses to build the project whenever a webhook is
received. The key thing to know is that the script expects to run inside a
Docker image where `/src` contains the repo code and `/build` is where the
output of the build goes. The Docker image doesn't have to be set up in any
special way. It just needs to have all the necessary build dependencies
installed. `/src` and `/build` are mounted automatically by quarantest.

Here's an example build script:

```bash
#!/bin/bash

cd /src
npm install
npm run build
cp -a * /build/
```

Here's another one for a very simple app that's just an index.html and
main.js, with no complicated build step:

```/bash
cp /src/* /build/*
```

quarantest will automatically download whatever Docker image you specify in
each repo.

You can find a real-world example in the gene.iobio repo [here][1].


# Set up the webhooks

Finally, you just need to point the GitHub webhooks at the `/webhook` endpoint
of the quarantest server.


[0]: https://github.com/iobio/gene.iobio.vue/pull/497 

[1]: https://github.com/iobio/gene.iobio.vue
