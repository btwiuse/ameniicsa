package commands

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/btwiuse/ameniicsa/api"
	"github.com/btwiuse/ameniicsa/asciicast"
	"github.com/btwiuse/ameniicsa/util"
)

type RecordCommand struct {
	API      api.API
	Env      map[string]string
	Recorder asciicast.Recorder
}

func NewRecordCommand(api api.API, env map[string]string) *RecordCommand {
	return &RecordCommand{
		API:      api,
		Env:      env,
		Recorder: asciicast.NewRecorder(),
	}
}

func (c *RecordCommand) Execute(command, title string, assumeYes bool, maxWait float64, filename string) error {
	var upload bool
	var err error

	if filename != "" {
		upload = false
	} else {
		filename, err = tmpPath()
		if err != nil {
			return err
		}
		upload = true
	}

	err = c.Recorder.Record(filename, command, title, maxWait, assumeYes, c.Env)
	if err != nil {
		return err
	}

	if upload {
		if !assumeYes {
			util.Printf("Press <Enter> to upload, <Ctrl-C> to cancel.")
			util.ReadLine()
		}

		var url, warn string
		var err error

		util.WithSpinner(0, func() {
			url, warn, err = c.API.UploadAsciicast(filename)
		})

		if warn != "" {
			util.Warningf(warn)
		}

		if err != nil {
			util.Warningf("Upload failed, asciicast saved at %v", filename)
			util.Warningf("Retry later by executing: asciinema upload %v", filename)
			return err
		}

		os.Remove(filename)
		fmt.Println(url)
	}

	return nil
}

func tmpPath() (string, error) {
	file, err := ioutil.TempFile("", "asciicast-")
	if err != nil {
		return "", err
	}
	defer file.Close()

	return file.Name(), nil
}
