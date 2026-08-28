package v1alpha1

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/yaml"
)

// checkedManifestDirs are the directories of user-facing manifests that are shipped with
// the repository. They are not applied by any test or CI job, so without this check they
// silently rot whenever the API changes.
var checkedManifestDirs = []string{"../../config/samples", "../../examples"}

// validateManifest decodes raw into the kind it declares and runs the same validator the
// admission webhook would. Unknown fields are rejected, so a manifest still using a
// removed or renamed field fails here.
func validateManifest(raw []byte) error {
	switch content := string(raw); {
	case strings.Contains(content, "kind: BlackboxExporter"):
		var o BlackboxExporter
		if err := yaml.UnmarshalStrict(raw, &o); err != nil {
			return err
		}
		_, err := validateExporterSpec(&o.Spec)
		return err
	case strings.Contains(content, "kind: BlackboxModule"):
		var o BlackboxModule
		if err := yaml.UnmarshalStrict(raw, &o); err != nil {
			return err
		}
		_, err := validateModuleSpec(&o.Spec)
		return err
	case strings.Contains(content, "kind: BlackboxProbe"):
		var o BlackboxProbe
		if err := yaml.UnmarshalStrict(raw, &o); err != nil {
			return err
		}
		_, err := validateProbeSpec(&o.Spec)
		return err
	default:
		// Kustomizations, Secrets, Ingresses and other supporting manifests.
		return nil
	}
}

func TestShippedManifestsAreValid(t *testing.T) {
	var checked int

	for _, dir := range checkedManifestDirs {
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".yaml") {
				return err
			}

			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !strings.Contains(string(raw), "apiVersion: monitoring.gaiser.bayern/") {
				return nil
			}

			checked++
			if err := validateManifest(raw); err != nil {
				t.Errorf("%s: %v", path, err)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}

	// Guard against the walk silently matching nothing, e.g. after a directory move.
	if checked < 10 {
		t.Fatalf("only %d manifests checked, expected the samples plus the examples", checked)
	}
	t.Logf("validated %d manifests", checked)
}
