# smith.yaml

## skeleton

```yaml
type: "" # mock, container
mock:
  config: "" # config file to use for mock
    pre-build: "" # command to run before running mock
    post-build: "" # command to run after running mock
    deps: [""] # list of dependencies to install for build
    debuginfo: false # whether to generate debug info for packages
    debugdeps: [""] # list of dependencies required when debuginfo enabled
    debugpaths: [""] # list of paths to include when debuginfo is enabled
package: "" # primary package name, or file name of archive for oci and docker
paths: [""] # list of paths to include in container
excludes: [""] # list of paths to exclude from container
parent: "" # name of parent image for layering
nss: false # whether to include common nss libraries by default
root: false # don't run the container command as a different user
user: "" # username for running the process as
groups: [""] # list of groups the user should be in
mounts: [""] # list of exported mounts/volumes
entrypoint: [""] # entrypoint of image
cmd: [""] # command of image
dir: "" # working directory for process
env: [""] # environment to use for process
labels:
  labelKey: "" # label the container with these values
ports: {} # expose these ports on the container
```
