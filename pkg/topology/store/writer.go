package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/ldez/go-git-cmd-wrapper/v2/add"
	"github.com/ldez/go-git-cmd-wrapper/v2/branch"
	"github.com/ldez/go-git-cmd-wrapper/v2/checkout"
	"github.com/ldez/go-git-cmd-wrapper/v2/commit"
	"github.com/ldez/go-git-cmd-wrapper/v2/git"
	"github.com/ldez/go-git-cmd-wrapper/v2/pull"
	"github.com/ldez/go-git-cmd-wrapper/v2/push"
	"github.com/rs/zerolog/log"
	"github.com/traefik/neo-agent/pkg/topology/state"
)

// Write writes the given cluster state in the current git repository.
func (s *Store) Write(ctx context.Context, st *state.Cluster) error {
	output, err := git.Branch(branch.List, branch.Format("%(refname:short)"), git.CmdExecutor(s.gitExecutor))
	if err != nil {
		return fmt.Errorf("list branches: %w %s", err, output)
	}

	if strings.Contains(output, st.ID) {
		// The branch already exists.
		output, err = git.CheckoutWithContext(ctx, checkout.Branch(st.ID), git.CmdExecutor(s.gitExecutor))
		if err != nil {
			return fmt.Errorf("checkout local branch: %w %s", err, output)
		}
	} else {
		// Creating new branch from checkout.
		output, err = git.CheckoutWithContext(ctx, checkout.NewBranch(st.ID), git.CmdExecutor(s.gitExecutor))
		if err != nil {
			return fmt.Errorf("checkout new local branch: %w %s", err, output)
		}

		output, err = git.PullWithContext(ctx, pull.FfOnly, pull.Repository("origin"), pull.Refspec(st.ID), git.CmdExecutor(s.gitExecutor))
		if err != nil && !strings.Contains(output, fmt.Sprintf("couldn't find remote ref %s", st.ID)) {
			return fmt.Errorf("git pull: %w: %s", err, output)
		}
	}

	err = s.write(st)
	if err != nil {
		return err
	}

	output, err = git.AddWithContext(ctx, add.PathSpec("./"), git.CmdExecutor(s.gitExecutor))
	if err != nil {
		return fmt.Errorf("git add: %w: %s", err, output)
	}

	output, err = git.CommitWithContext(ctx, commit.Message(time.Now().String()), git.CmdExecutor(s.gitExecutor))
	if err != nil {
		if strings.Contains(output, "nothing to commit") {
			return nil
		}

		return fmt.Errorf("git commit: %w: %s", err, output)
	}

	output, err = git.PushWithContext(ctx, push.All, push.SetUpstream, git.CmdExecutor(s.gitExecutor))
	if err != nil {
		return fmt.Errorf("git push: %w: %s", err, output)
	}

	return nil
}

// write writes the cluster resource into files.
// It uses reflect to have a common way to create a file tree.
// For each public cluster field a directory is created with the field name.
// For each supported types (map, slice, string) a sub function creates files in this directory.
func (s *Store) write(st *state.Cluster) error {
	if st == nil {
		return nil
	}

	if err := cleanDir(s.workingDir); err != nil {
		return fmt.Errorf("clean dir: %w", err)
	}

	t := reflect.TypeOf(*st)
	v := reflect.ValueOf(*st)
	for i := 0; i < t.NumField(); i++ {
		switch t.Field(i).Type.Kind() {
		case reflect.Map:
			err := s.writeMap(t.Field(i), v.Field(i))
			if err != nil {
				return err
			}
		case reflect.Slice:
			err := s.writeSlice(t.Field(i), v.Field(i))
			if err != nil {
				return err
			}
		case reflect.String:
			err := s.writeString(t.Field(i), v.Field(i))
			if err != nil {
				return err
			}
		case reflect.Struct:
			err := s.writeStruct(t.Field(i), v.Field(i))
			if err != nil {
				return err
			}
		default:
			log.Error().Str("kind", t.Field(i).Type.Kind().String()).Msg("unrecognized kind")
		}
	}

	return nil
}

// writeMap marshals each map value and writes it to a file.
// It uses the following path pattern: field.Name/value (e.g.: Ingresses/myingress@default.json).
func (s *Store) writeMap(field reflect.StructField, value reflect.Value) error {
	dir := field.Name

	switch tag := field.Tag.Get("dir"); tag {
	case "":
		// Use the field name.
	case "-":
		return nil
	default:
		dir = tag
	}

	for _, index := range value.MapKeys() {
		val := reflect.Indirect(value.MapIndex(index))

		data, err := json.MarshalIndent(val.Interface(), "", "\t")
		if err != nil {
			return fmt.Errorf("marshal resource: %s %w", index, err)
		}

		fileName := fmt.Sprintf("%s/%s.json", dir, index)
		if err = writeFile(filepath.Join(s.workingDir, fileName), data); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
	}

	return nil
}

// writeSlice marshals each slice value and writes it to a file.
// It uses the following path pattern: field.Name/value (e.g.: Namespaces/default).
func (s *Store) writeSlice(field reflect.StructField, value reflect.Value) error {
	dir := field.Name
	if field.Tag.Get("dir") != "" {
		dir = field.Tag.Get("dir")
	}

	for i := 0; i < value.Len(); i++ {
		fileName := fmt.Sprintf("%s/%s", dir, value.Index(i))

		err := writeFile(filepath.Join(s.workingDir, fileName), []byte(value.Index(i).String()))
		if err != nil {
			return fmt.Errorf("write file: %w", err)
		}
	}

	return nil
}

// writeString writes a string value to a file (field.Name).
func (s *Store) writeString(field reflect.StructField, value reflect.Value) error {
	fileName := field.Name

	err := writeFile(filepath.Join(s.workingDir, fileName), []byte(value.String()))
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// writeStruct writes a struct value to a file (field.Name).
func (s *Store) writeStruct(field reflect.StructField, value reflect.Value) error {
	data, err := json.MarshalIndent(value.Interface(), "", "\t")
	if err != nil {
		return fmt.Errorf("marshal resource: %w", err)
	}

	fileName := fmt.Sprintf("%s.json", field.Name)
	err = writeFile(filepath.Join(s.workingDir, fileName), data)
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

func writeFile(filePath string, data []byte) error {
	dir := filepath.Dir(filePath)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
	}

	return os.WriteFile(filePath, data, 0o600)
}

func cleanDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.Name() == ".git" || entry.Name() == "README.md" {
			continue
		}

		if err = os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}

	return nil
}
