package docker

import (
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"
)

const (
	sockPath         = "test.sock"
	testfileLocation = "testfiles/"
)

type daemonMock struct {
	StatusCode int
	Response   []byte
	sock       net.Listener
}

func (d *daemonMock) Listen() error {
	var err error
	d.sock, err = net.Listen("unix", sockPath)
	if err != nil {
		return err
	}
	go func() {
		http.Serve(d.sock, http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				if d.StatusCode != 0 && d.StatusCode != http.StatusOK {
					w.WriteHeader(d.StatusCode)
				}
				w.Header().Add("Content-Type", "application/json")
				w.Write(d.Response)
			}))
	}()
	return nil
}

func (d *daemonMock) Close() error {
	return d.sock.Close()
}

var (
	client *Client
	srv    *daemonMock
)

func TestMain(m *testing.M) {
	client = NewClient(sockPath)
	srv = &daemonMock{StatusCode: http.StatusOK}

	if err := syscall.Unlink(sockPath); err != nil {
		println(err.Error())
	}

	if err := srv.Listen(); err != nil {
		println(err.Error())
		os.Exit(1)
	}

	// NOTE: wait for "test.sock" (10 retries, sleep for 1 sec)
	var err error
	for i := 0; i < 10; i++ {
		if _, err = os.Stat(sockPath); err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}

	rc := m.Run()
	if err := srv.Close(); err != nil {
		println(err.Error())
	}
	os.Exit(rc)
}

func Test_ContainerIDByName(t *testing.T) {
	tt := []struct {
		name          string
		containerName string
		responseFile  string
		expect        string
		wantErr       bool
	}{
		{
			name:          "expected",
			containerName: "house",
			responseFile:  "containers.json",
			expect:        "60a2038405bb0bdbb1fd75d1cec9dadbdc328fe9d340546cbc75f7c2e01d57ed",
		},
		{
			name:          "not in list",
			containerName: "not_in_list",
			responseFile:  "containers.json",
			wantErr:       true,
		},
		{
			name:         "fail",
			responseFile: "empty.json",
			wantErr:      true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			path := testfileLocation + tc.responseFile
			if srv.Response, err = ioutil.ReadFile(path); err != nil {
				t.Error(err)
			}
			id, err := client.ContainerIDByName(tc.containerName)
			if err != nil && !tc.wantErr {
				t.Error(err)
			}
			if id != tc.expect && !tc.wantErr {
				t.Errorf("got: %s, want: %s", id, tc.expect)
			}
		})
	}
}

func Test_NetworkIDByName(t *testing.T) {

	tt := []struct {
		name         string
		networkName  string
		responseFile string
		expect       string
		wantErr      bool
	}{
		{
			name:         "expected",
			networkName:  "simulation_subnet_1",
			responseFile: "networks.json",
			expect:       "422bb11698f5f30491ec100674f1baf46ea360bef19fed498d6dc40b9b5c2ca7",
		},
		{
			name:         "not in list",
			networkName:  "not_in_list",
			responseFile: "networks.json",
			wantErr:      true,
		},
		{
			name:         "fail",
			responseFile: "empty.json",
			wantErr:      true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			path := testfileLocation + tc.responseFile
			if srv.Response, err = ioutil.ReadFile(path); err != nil {
				t.Error(err)
			}
			id, err := client.NetworkIDByName(tc.networkName)
			if err != nil && !tc.wantErr {
				t.Error(err)
			}
			if id != tc.expect && !tc.wantErr {
				t.Errorf("got: %s, want: %s", id, tc.expect)
			}
		})
	}
}

func Test_CreateContainer(t *testing.T) {

	tt := []struct {
		name         string
		image        string
		responseFile string
		statusCode   int
		expect       string
		wantErr      bool
	}{
		{
			name:         "expected",
			image:        "simulation_subnet_1",
			responseFile: "containers_create.json",
			statusCode:   http.StatusCreated,
			expect:       "123456",
		},
		{
			name:         "fail",
			responseFile: "empty.json",
			wantErr:      true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			srv.StatusCode = 0
			var err error
			if tc.statusCode != 0 {
				srv.StatusCode = tc.statusCode
			}

			path := testfileLocation + tc.responseFile
			if srv.Response, err = ioutil.ReadFile(path); err != nil {
				t.Error(err)
			}
			id, err := client.CreateContainer(tc.name, tc.image, []string{}, []string{}, []string{})
			if err != nil && !tc.wantErr {
				t.Error(err)
			}
			if id != tc.expect && !tc.wantErr {
				t.Errorf("got: %s, want: %s", id, tc.expect)
			}
		})
	}
}

func Test_CreateNetwork(t *testing.T) {

	tt := []struct {
		name         string
		image        string
		responseFile string
		statusCode   int
		expect       string
		wantErr      bool
	}{
		{
			name:         "expected",
			image:        "simulation_subnet_1",
			responseFile: "networks_create.json",
			statusCode:   http.StatusCreated,
			expect:       "2345",
		},
		{
			name:         "fail",
			responseFile: "empty.json",
			wantErr:      true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			srv.StatusCode = 0
			var err error
			if tc.statusCode != 0 {
				srv.StatusCode = tc.statusCode
			}
			path := testfileLocation + tc.responseFile
			if srv.Response, err = ioutil.ReadFile(path); err != nil {
				t.Error(err)
			}
			id, err := client.CreateNetwork(tc.name)
			if err != nil && !tc.wantErr {
				t.Error(err)
			}
			if id != tc.expect && !tc.wantErr {
				t.Errorf("got: %s, want: %s", id, tc.expect)
			}
		})
	}
}

func Test_Labels(t *testing.T) {
	tt := []struct {
		name         string
		containerID  string
		responseFile string
		expect       map[string]string
		wantErr      bool
	}{
		{
			name:         "expected",
			containerID:  "1234",
			responseFile: "inspect.json",
			expect: map[string]string{
				"com.example.license": "GPL",
				"com.example.vendor":  "Acme",
				"com.example.version": "1.0",
			},
		},
		{
			name:         "fail",
			responseFile: "empty.json",
			wantErr:      true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			path := testfileLocation + tc.responseFile
			if srv.Response, err = ioutil.ReadFile(path); err != nil {
				t.Error(err)
			}
			got, err := client.Labels(tc.containerID)
			if err != nil && !tc.wantErr {
				t.Error(err)
			}
			for k, v := range tc.expect {
				res, ok := got[k]
				if !ok {
					t.Errorf("missing key %s", k)
					continue
				}
				if v != res {
					t.Errorf("invalid value for key %s. got: %s, want: %s", k, res, v)
				}
			}
		})
	}
}
