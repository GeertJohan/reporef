## reporef

This service allows anyone to reference a certain branch or commit in a public git repository (as hosted by a popular git provider), by URL.
For instance: `reporef.com/github.com/GeertJohan/myProject@release` will point to the last commit at the `release` branch of the `myProject` repository.

Functionality is far from complete. On the TODO list are:
- Good interface, with links to the original project and other branches.
- Special information in interface for public Go packages (e.g.:  link to godoc.org)
- Complete testing and enhancement of the way the cloning and re-master process works.