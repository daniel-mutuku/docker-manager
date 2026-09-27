package web

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"golang.org/x/crypto/bcrypt"

	"docker-manager/internal/config"
	"docker-manager/internal/docker"
	"docker-manager/internal/store"
)

// fakeDockerService is an in-memory DockerService double, so handler tests
// don't need a live Docker daemon. Only ContainerService behavior is
// actually exercised by name-based fakes below; the Image/Volume/Network
// methods are no-ops that satisfy the interface.
type fakeDockerService struct {
	containers map[string]docker.ContainerSummary
	images     map[string]docker.ImageSummary
	volumes    map[string]docker.VolumeSummary
	networks   map[string]docker.NetworkSummary
}

func newFakeDockerService() *fakeDockerService {
	return &fakeDockerService{
		containers: map[string]docker.ContainerSummary{
			"c1": {ID: "c1", ShortID: "c1", Name: "web", Image: "nginx", State: "running", Status: "Up 2 minutes"},
		},
		images:   map[string]docker.ImageSummary{},
		volumes:  map[string]docker.VolumeSummary{},
		networks: map[string]docker.NetworkSummary{},
	}
}

func (f *fakeDockerService) ListContainers(ctx context.Context) ([]docker.ContainerSummary, error) {
	summaries := make([]docker.ContainerSummary, 0, len(f.containers))
	for _, c := range f.containers {
		summaries = append(summaries, c)
	}
	return summaries, nil
}

func (f *fakeDockerService) GetContainerSummary(ctx context.Context, id string) (docker.ContainerSummary, error) {
	c, ok := f.containers[id]
	if !ok {
		return docker.ContainerSummary{}, docker.ErrContainerNotFound
	}
	return c, nil
}

func (f *fakeDockerService) ListContainersByComposeProject(ctx context.Context, project string) ([]docker.ContainerSummary, error) {
	return nil, nil
}

func (f *fakeDockerService) InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error) {
	if _, ok := f.containers[id]; !ok {
		return types.ContainerJSON{}, docker.ErrContainerNotFound
	}
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{ID: id, Name: "/" + f.containers[id].Name},
	}, nil
}

func (f *fakeDockerService) StartContainer(ctx context.Context, id string) error {
	c := f.containers[id]
	c.State = "running"
	f.containers[id] = c
	return nil
}

func (f *fakeDockerService) StopContainer(ctx context.Context, id string) error {
	c := f.containers[id]
	c.State = "exited"
	f.containers[id] = c
	return nil
}

func (f *fakeDockerService) RestartContainer(ctx context.Context, id string) error {
	return nil
}

func (f *fakeDockerService) RemoveContainer(ctx context.Context, id string, force bool) error {
	delete(f.containers, id)
	return nil
}

func (f *fakeDockerService) Logs(ctx context.Context, id string, tail string, follow bool) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (f *fakeDockerService) StreamStats(ctx context.Context, id string, fn func(docker.ContainerStats)) error {
	return nil
}

func (f *fakeDockerService) SnapshotStats(ctx context.Context, id string) (docker.ContainerStats, error) {
	return docker.ContainerStats{}, nil
}

func (f *fakeDockerService) DiskUsage(ctx context.Context) (docker.DiskUsageSummary, error) {
	return docker.DiskUsageSummary{}, nil
}

func (f *fakeDockerService) ListImages(ctx context.Context) ([]docker.ImageSummary, error) {
	images := make([]docker.ImageSummary, 0, len(f.images))
	for _, img := range f.images {
		images = append(images, img)
	}
	return images, nil
}

func (f *fakeDockerService) PullImage(ctx context.Context, ref string) (io.ReadCloser, error) {
	f.images[ref] = docker.ImageSummary{ID: ref, ShortID: ref, Tags: []string{ref}}
	return io.NopCloser(strings.NewReader(`{"status":"Pull complete"}` + "\n")), nil
}

func (f *fakeDockerService) RemoveImage(ctx context.Context, id string, force bool) error {
	delete(f.images, id)
	return nil
}

func (f *fakeDockerService) PruneDanglingImages(ctx context.Context) (docker.PruneImageResult, error) {
	return docker.PruneImageResult{}, nil
}

func (f *fakeDockerService) ListVolumes(ctx context.Context) ([]docker.VolumeSummary, error) {
	volumes := make([]docker.VolumeSummary, 0, len(f.volumes))
	for _, v := range f.volumes {
		volumes = append(volumes, v)
	}
	return volumes, nil
}

func (f *fakeDockerService) CreateVolume(ctx context.Context, name string) error {
	f.volumes[name] = docker.VolumeSummary{Name: name, Driver: "local"}
	return nil
}

func (f *fakeDockerService) RemoveVolume(ctx context.Context, name string, force bool) error {
	delete(f.volumes, name)
	return nil
}

func (f *fakeDockerService) ListNetworks(ctx context.Context) ([]docker.NetworkSummary, error) {
	networks := make([]docker.NetworkSummary, 0, len(f.networks))
	for _, n := range f.networks {
		networks = append(networks, n)
	}
	return networks, nil
}

func (f *fakeDockerService) GetNetwork(ctx context.Context, id string) (docker.NetworkSummary, error) {
	n, ok := f.networks[id]
	if !ok {
		return docker.NetworkSummary{}, fmt.Errorf("network %s not found", id)
	}
	return n, nil
}

func (f *fakeDockerService) CreateNetwork(ctx context.Context, name, driver string) error {
	if driver == "" {
		driver = "bridge"
	}
	f.networks[name] = docker.NetworkSummary{ID: name, ShortID: name, Name: name, Driver: driver}
	return nil
}

func (f *fakeDockerService) RemoveNetwork(ctx context.Context, id string) error {
	delete(f.networks, id)
	return nil
}

func (f *fakeDockerService) ExecShell(ctx context.Context, containerID string) (string, types.HijackedResponse, error) {
	return "", types.HijackedResponse{}, errors.New("exec not supported in tests")
}

func (f *fakeDockerService) ExecResize(ctx context.Context, execID string, height, width uint) error {
	return errors.New("exec not supported in tests")
}

// testServer wires a Server against a fake ContainerService and an
// in-memory store, and returns an httptest server plus the admin password
// needed to log in.
func testServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()

	const password = "hunter2"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hashing test password: %v", err)
	}

	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	cfg := &config.Config{
		AdminUsername:     "admin",
		AdminPasswordHash: string(hash),
	}

	handler, err := NewServer(cfg, newFakeDockerService(), st, t.TempDir())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, password
}

// loggedInClient returns an http.Client that has already authenticated
// against srv and will carry the session cookie on subsequent requests.
func loggedInClient(t *testing.T, srv *httptest.Server, password string) *http.Client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("creating cookie jar: %v", err)
	}
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.PostForm(srv.URL+"/login", map[string][]string{
		"username": {"admin"},
		"password": {password},
	})
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	return client
}

func TestUnauthenticatedRequestRedirectsToLogin(t *testing.T) {
	srv, _ := testServer(t)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	resp, err := client.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Fatalf("Location = %q, want /login", loc)
	}
}

func TestLogin_WrongPasswordIsRejected(t *testing.T) {
	srv, _ := testServer(t)

	resp, err := http.PostForm(srv.URL+"/login", map[string][]string{
		"username": {"admin"},
		"password": {"wrong"},
	})
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestLoginThenViewContainers(t *testing.T) {
	srv, password := testServer(t)
	client := loggedInClient(t, srv, password)

	resp, err := client.Get(srv.URL + "/containers")
	if err != nil {
		t.Fatalf("GET /containers: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body := new(strings.Builder)
	if _, err := io.Copy(body, resp.Body); err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if !strings.Contains(body.String(), "web") {
		t.Fatalf("expected the fake container's name in the response body, got: %s", body.String())
	}
}

func TestLogoutClearsSession(t *testing.T) {
	srv, password := testServer(t)
	client := loggedInClient(t, srv, password)

	resp, err := client.Post(srv.URL+"/logout", "application/x-www-form-urlencoded", nil)
	if err != nil {
		t.Fatalf("logout request: %v", err)
	}
	resp.Body.Close()

	resp, err = client.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET / after logout: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status after logout = %d, want redirect to login", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Fatalf("Location after logout = %q, want /login", loc)
	}
}

func TestVolumes_CreateThenList(t *testing.T) {
	srv, password := testServer(t)
	client := loggedInClient(t, srv, password)

	resp, err := client.PostForm(srv.URL+"/volumes", map[string][]string{"name": {"my-vol"}})
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("create status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}

	resp, err = client.Get(srv.URL + "/volumes")
	if err != nil {
		t.Fatalf("GET /volumes: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "my-vol") {
		t.Fatalf("expected the created volume in the list, got: %s", body)
	}
}

func TestVolumes_Remove(t *testing.T) {
	srv, password := testServer(t)
	client := loggedInClient(t, srv, password)

	resp, err := client.PostForm(srv.URL+"/volumes", map[string][]string{"name": {"my-vol"}})
	if err != nil {
		t.Fatalf("create volume: %v", err)
	}
	resp.Body.Close()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/volumes/my-vol", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("delete volume: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete status = %d, want 200", resp.StatusCode)
	}

	resp, err = client.Get(srv.URL + "/volumes")
	if err != nil {
		t.Fatalf("GET /volumes: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "my-vol") {
		t.Fatalf("removed volume still appears in the list: %s", body)
	}
}

func TestNetworks_CreateListAndViewDetail(t *testing.T) {
	srv, password := testServer(t)
	client := loggedInClient(t, srv, password)

	resp, err := client.PostForm(srv.URL+"/networks", map[string][]string{"name": {"my-net"}})
	if err != nil {
		t.Fatalf("create network: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("create status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}

	resp, err = client.Get(srv.URL + "/networks")
	if err != nil {
		t.Fatalf("GET /networks: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "my-net") {
		t.Fatalf("expected the created network in the list, got: %s", body)
	}

	resp, err = client.Get(srv.URL + "/networks/my-net")
	if err != nil {
		t.Fatalf("GET /networks/my-net: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("detail status = %d, want 200", resp.StatusCode)
	}
}

func TestImages_ListShowsPulledImage(t *testing.T) {
	srv, password := testServer(t)
	client := loggedInClient(t, srv, password)

	resp, err := client.PostForm(srv.URL+"/images/pull", map[string][]string{"ref": {"alpine:latest"}})
	if err != nil {
		t.Fatalf("pull image: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	resp, err = client.Get(srv.URL + "/images")
	if err != nil {
		t.Fatalf("GET /images: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "alpine:latest") {
		t.Fatalf("expected the pulled image in the list, got: %s", body)
	}
}

func TestStacks_CreateThenList(t *testing.T) {
	srv, password := testServer(t)
	client := loggedInClient(t, srv, password)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("name", "my-stack")
	writer.WriteField("compose_content", "services:\n  web:\n    image: nginx\n")
	writer.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/stacks", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("create stack: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("create status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}

	resp, err = client.Get(srv.URL + "/stacks")
	if err != nil {
		t.Fatalf("GET /stacks: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "my-stack") {
		t.Fatalf("expected the created stack in the list, got: %s", body)
	}
}

func TestStacks_CreateWithInvalidNameRejected(t *testing.T) {
	srv, password := testServer(t)
	client := loggedInClient(t, srv, password)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("name", "Not A Valid Name")
	writer.WriteField("compose_content", "services: {}")
	writer.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/stacks", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("create stack: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d for an invalid stack name", resp.StatusCode, http.StatusBadRequest)
	}
}
