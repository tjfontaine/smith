package main

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	gdigest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

type dockerSaveManifest struct {
	ConfigFileName string   `json:"Config"`
	Layers         []string `json:"Layers"`
	RepoTags       []string `json:"RepoTags"`
}

func getDockerManifest(manifestJson []byte, inArchive string) (*dockerSaveManifest, error) {
	var dockerManifest []dockerSaveManifest

	var err error

	if manifestJson == nil {
		manifestJson, err = extractFile(inArchive, "manifest.json")
		if err != nil {
			return nil, fmt.Errorf("failed to extract docker archive manifest %s: %v", inArchive, err)
		}
	}

	if err := json.Unmarshal(manifestJson, &dockerManifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal docker archive manifest: %v", err)
	}

	if len(dockerManifest) > 1 {
		return nil, fmt.Errorf("only archives with a single image are supported")
	}

	return &dockerManifest[0], nil
}

type DockerArchiveType struct {
}

func (at DockerArchiveType) Name() string {
	return "docker"
}

func (at DockerArchiveType) Probe(inArchive string) (bool, []byte) {
	refb, err := extractFile(inArchive, "manifest.json")
	if err != nil {
		logrus.Debugf("failed to find docker archive: %v", err)
		return false, nil
	}
	return true, refb
}

func getDockerImageConfig(inArchive string, configFile string) (*v1.Image, error) {
	configJson, err := extractFile(inArchive, configFile)

	if err != nil {
		logrus.Fatalf("Failed to extract config file %s from docker archive %s: %v", configFile, inArchive, err)
		return nil, err
	}

	var imageConfig v1.Image

	if err = json.Unmarshal(configJson, &imageConfig); err != nil {
		logrus.Fatalf("Failed to unmarshal docker image config: %v", err)
		return nil, err
	}

	logrus.Debugf("docker image config: %+v", imageConfig)

	return &imageConfig, nil
}

func (at DockerArchiveType) GetImage(manifestBytes []byte, tag string, shortPath string, inArchive string) (*Image, error) {
	dockerManifest, err := getDockerManifest(manifestBytes, inArchive)

	if err != nil {
		logrus.Fatalf("Failed to get manifest: %v", err)
		return nil, err
	}

	logrus.Debugf("docker archive manifest: %v", dockerManifest)

	imageConfig, err := getDockerImageConfig(inArchive, dockerManifest.ConfigFileName)

	if err != nil {
		logrus.Fatalf("Failed to unmarshal docker image config: %v", err)
		return nil, err
	}

	logrus.Debugf("docker image config: %+v", imageConfig)

	layers := make([]*Layer, len(dockerManifest.Layers))
	logrus.Debugf("layers: %v", layers)

	for index, layer := range dockerManifest.Layers {
		layerContent, err := extractFile(inArchive, layer)

		logrus.Debugf("Extracting layer %s", layer)

		if err != nil {
			logrus.Errorf("Failed to extract layer %s -> %s: %v", inArchive, layer, err)
			return nil, err
		}

		shaString := strings.Split(layer, "/")[0]

		logrus.Debugf("layer sha string %s", shaString)

		digest := gdigest.NewDigestFromHex("sha256", shaString)

		logrus.Debugf("made digest: %s - %s", digest.Hex(), digest.String())

		newLayer := &Layer{
			Data: layerContent,
			Desc: v1.Descriptor{
				MediaType: v1.MediaTypeImageLayer,
				Digest:    digest,
				Size:      int64(len(layerContent)),
			},
		}

		layers[index] = newLayer

		logrus.Debugf("new layer appended: %v", layers)
	}

	logrus.Debugf("extracted all layers: %v", layers)

	image := Image{
		Config:   imageConfig,
		Metadata: getMetadata(),
		Layers:   layers,
	}

	return &image, nil
}

func WriteDockerFromBuild(buildOpts *buildOptions, def *ConfigDef, buildDir, outName string, metadata *ImageMetadata, blobs []OpaqueBlob) error {
	image, err := imageFromBuild(def, buildDir)

	if err != nil {
		return err
	}

	image.AdditionalBlobs = blobs

	if metadata != nil {
		image.Metadata = metadata
	}

	logrus.Debugf("Image from build %s from %v", outName, image)

	tempFile, err := ioutil.TempFile("", "docker-archive")
	defer tempFile.Close()

	success := false

	defer func() {
		if !success {
			logrus.Debugf("Deferred remove happening")
			os.Remove(tempFile.Name())
		}
	}()

	if err != nil {
		logrus.Fatalf("Couldn't open temporary file for docker archive: %v", err)
		return err
	}

	err = WriteDockerTarGz(buildOpts, image, tempFile)

	tempFile.Close()

	if err != nil {
		logrus.Fatalf("failed to write docker tar.gz: %v", err)
		return err
	}

	logrus.Debugf("renaming %s -> %s", tempFile.Name(), outName)

	err = Copy(tempFile.Name(), outName)

	if err != nil {
		logrus.Fatalf("Failed to rename archive: %v", err)
		return err
	}

	os.Remove(tempFile.Name())

	success = true

	return nil
}

func WriteDockerTarGz(buildOpts *buildOptions, image *Image, out io.Writer) error {
	gzipOut, err := MaybeGzipWriter(out)
	if err != nil {
		return err
	}
	if err := WriteDockerTar(buildOpts, image, gzipOut); err != nil {
		logrus.Errorf("Error writing docker tar.gz: %v", err)
		gzipOut.Close()
		return err
	}
	// explicitly close the gzip so we wait for the write to complete
	gzipOut.Close()
	return nil
}

func WriteDockerTar(buildOpts *buildOptions, image *Image, out io.Writer) error {
	destTar := tar.NewWriter(out)

	/* We could add a history item, but the archive won't be reproducible

	curTime := time.Now()
	metadataComment, err := json.Marshal(getMetadata())

	if err != nil {
		logrus.Warnf("failed to serialize metadata for comment: %v", err)
		metadataComment = []byte{}
	}

	image.Config.History = []v1.History{{
		Created:   &curTime,
		CreatedBy: "smith",
		Comment:   string(metadataComment),
	}}
	*/

	manifest := dockerSaveManifest{
		Layers: []string{},
		RepoTags: []string{}, // we need to synthesize this tag?
	}

	if buildOpts.tag != "" {
		manifest.RepoTags = append(manifest.RepoTags, buildOpts.tag)
	}

	for _, layer := range image.Layers {
		err := writeDirTar(destTar, layer.Desc.Digest.Hex())

		if err != nil {
			logrus.Fatalf("Failed to add directory to destination tar: %v", err)
			return err
		}

		err = writeFileTar(destTar, layer.Desc.Digest.Hex()+"/layer.tar", layer.Data)

		if err != nil {
			logrus.Fatalf("Failed to add file %s to destination archive: %v", layer.Desc.Digest.Hex()+"/layer.tar", err)
			return err
		}

		manifest.Layers = append(manifest.Layers, layer.Desc.Digest.Hex()+"/layer.tar")

		image.Config.RootFS.DiffIDs = append(image.Config.RootFS.DiffIDs, layer.DiffID)

		if err != nil {
			logrus.Fatalf("Failed to write layer: %v", err)
			return err
		}
	}

	configStr, err := json.Marshal(image.Config)

	if err != nil {
		logrus.Fatalf("Failed to serialize docker config: %v", err)
		return err
	}

	configSha := gdigest.FromBytes(configStr)

	logrus.Debugf("configStr: %s", configStr)

	if err != nil {
		logrus.Fatalf("Failed to write docker config: %v", err)
		return err
	}

	manifest.ConfigFileName = configSha.Hex() + ".json"

	manifestStr, err := json.Marshal([]dockerSaveManifest{manifest})

	logrus.Debugf("manifestStr: %s", manifestStr)

	err = writeFileTar(destTar, "manifest.json", manifestStr)

	if err != nil {
		logrus.Fatalf("Failed to commit writing docker-archive tarfile: %v", err)
		return err
	}

	err = writeFileTar(destTar, manifest.ConfigFileName, configStr)

	if err != nil {
		logrus.Fatalf("Failed to add config file %s for archive: %v", manifest.ConfigFileName, err)
		return err
	}

	destTar.Close()
	return nil
}
