/*
Copyright © 2022 Guilhem Lettron

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/guilhem/chunkreaderat"
	"github.com/guilhem/gorkers"
	"github.com/snabb/httpreaderat"
	"github.com/spf13/cobra"
)

// zipCmd represents the zip command
var zipCmd = &cobra.Command{
	Use:   "zip",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: unzip,
}

func init() {
	rootCmd.AddCommand(zipCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// zipCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// zipCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func unzip(cmd *cobra.Command, args []string) error {
	res, ok := cmd.Context().Value(httpreaderat.HTTPReaderAt{}).(*httpreaderat.HTTPReaderAt)
	if !ok {
		return errors.New("can't get httpreaderat.HTTPReaderAt")
	}

	cReader, err := chunkreaderat.NewChunkReaderAt(res, res.Size(), 1024*1024, 100)
	if err != nil {
		return err
	}

	zfile, err := zip.NewReader(cReader, cReader.Size())
	if err != nil {
		return err
	}

	work := func(ctx context.Context, in *zip.File, out chan<- interface{}) error {
		return unzipFile(in, destination)
	}

	runner := gorkers.NewRunner(cmd.Context(), work, int64(concurent), 1)

	logf := func(ctx context.Context, in *zip.File, err error) error {
		if err != nil {
			log.Printf("err: %s", err)
		}
		return nil
	}
	runner.AfterFunc(logf)

	if err := runner.Start(); err != nil {
		return nil
	}

	defer runner.Stop()

	for _, f := range zfile.File {
		log.Printf("unzip file %s", f.Name)
		if err := runner.Send(f); err != nil {
			return err
		}
	}

	runner.Wait().Stop()

	return nil
}

func unzipFile(f *zip.File, destination string) error {
	// 4. Check if file paths are not vulnerable to Zip Slip
	filePath := filepath.Join(destination, f.Name)
	// if !strings.HasPrefix(filePath, filepath.Clean(destination)+string(os.PathSeparator)) {
	// 	return fmt.Errorf("invalid file path: %s", filePath)
	// }

	// 5. Create directory tree
	if f.FileInfo().IsDir() {
		if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return err
	}

	// 6. Create a destination file for unzipped content
	destinationFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	// 7. Unzip the content of a file and copy it to the destination file
	zippedFile, err := f.Open()
	if err != nil {
		return err
	}
	defer zippedFile.Close()

	if _, err := io.Copy(destinationFile, zippedFile); err != nil {
		return err
	}

	return nil
}
