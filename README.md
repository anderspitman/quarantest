Most CI testing tools focus on automated tests, but sometimes the changes are
very visual and you just want to give your team a demo of your pull request to
play with. quarantest runs a build for each GitHub PR, generates a URL for the
build, then posts a comment on the PR with a link to the build. You can see an
example of it in action [here][0]. Still in a pretty hacky state. Probably would
be better to use the GH status API with a link that goes to a page listing all
the past builds from the PR instead of spamming comments, but it's getting the
job done.

[0]: https://github.com/anderspitman/quarantest
