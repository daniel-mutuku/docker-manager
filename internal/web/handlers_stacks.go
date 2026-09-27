package web

import (
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"docker-manager/internal/compose"
	"docker-manager/internal/docker"
	"docker-manager/internal/store"
)

type stackRow struct {
	store.Stack
	Containers []docker.ContainerSummary
}

type stacksListData struct {
	PageData
	Stacks []stackRow
	Error  string
}

func (s *Server) handleStacksList(w http.ResponseWriter, r *http.Request) {
	stacks, err := s.store.ListStacks(r.Context())
	data := stacksListData{PageData: s.pageData(r, "stacks")}
	if err != nil {
		data.Error = "Could not list stacks: " + err.Error()
		s.renderer.Render(w, http.StatusOK, "stacks_list", data)
		return
	}

	for _, st := range stacks {
		containers, err := s.docker.ListContainersByComposeProject(r.Context(), st.Name)
		if err != nil {
			data.Error = "Could not load container status: " + err.Error()
		}
		data.Stacks = append(data.Stacks, stackRow{Stack: st, Containers: containers})
	}

	s.renderer.Render(w, http.StatusOK, "stacks_list", data)
}

type stackNewPageData struct {
	PageData
	Error string
}

func (s *Server) handleStackNewPage(w http.ResponseWriter, r *http.Request) {
	s.renderer.Render(w, http.StatusOK, "stack_new", stackNewPageData{PageData: s.pageData(r, "stacks")})
}

// maxStackUploadBytes caps a pasted/uploaded compose file; real-world
// compose files are a few KB, so this is generous headroom, not a target.
const maxStackUploadBytes = 1 << 20 // 1 MiB

func (s *Server) handleStackCreate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxStackUploadBytes)
	if err := r.ParseMultipartForm(maxStackUploadBytes); err != nil {
		s.renderer.Render(w, http.StatusBadRequest, "stack_new", stackNewPageData{
			PageData: s.pageData(r, "stacks"),
			Error:    "Could not parse form: " + err.Error(),
		})
		return
	}

	name := r.FormValue("name")
	content := r.FormValue("compose_content")

	if file, _, err := r.FormFile("compose_file"); err == nil {
		defer file.Close()
		raw, err := io.ReadAll(io.LimitReader(file, maxStackUploadBytes))
		if err != nil {
			s.renderer.Render(w, http.StatusBadRequest, "stack_new", stackNewPageData{
				PageData: s.pageData(r, "stacks"),
				Error:    "Could not read uploaded file: " + err.Error(),
			})
			return
		}
		content = string(raw)
	}

	if err := compose.ValidateName(name); err != nil {
		s.renderer.Render(w, http.StatusBadRequest, "stack_new", stackNewPageData{
			PageData: s.pageData(r, "stacks"),
			Error:    err.Error(),
		})
		return
	}
	if content == "" {
		s.renderer.Render(w, http.StatusBadRequest, "stack_new", stackNewPageData{
			PageData: s.pageData(r, "stacks"),
			Error:    "Paste or upload a docker-compose.yml",
		})
		return
	}

	if _, err := s.store.CreateStack(r.Context(), name, content); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrStackExists) {
			status = http.StatusConflict
		}
		s.renderer.Render(w, status, "stack_new", stackNewPageData{
			PageData: s.pageData(r, "stacks"),
			Error:    err.Error(),
		})
		return
	}

	http.Redirect(w, r, "/stacks/"+name, http.StatusSeeOther)
}

type stackDetailData struct {
	PageData
	Stack      store.Stack
	Containers []docker.ContainerSummary
	Output     string
	Error      string
}

func (s *Server) handleStackDetail(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	s.renderStackDetail(w, r, name, "", "")
}

func (s *Server) renderStackDetail(w http.ResponseWriter, r *http.Request, name, output, opErr string) {
	stack, err := s.store.GetStack(r.Context(), name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	containers, err := s.docker.ListContainersByComposeProject(r.Context(), name)
	if err != nil && opErr == "" {
		opErr = "Could not load container status: " + err.Error()
	}

	s.renderer.Render(w, http.StatusOK, "stack_detail", stackDetailData{
		PageData:   s.pageData(r, "stacks"),
		Stack:      *stack,
		Containers: containers,
		Output:     output,
		Error:      opErr,
	})
}

func (s *Server) handleStackDeploy(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	stack, err := s.store.GetStack(r.Context(), name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	out, err := s.compose.Up(r.Context(), name, stack.ComposeContent)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	s.renderStackDetail(w, r, name, string(out), errMsg)
}

func (s *Server) handleStackDown(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	out, err := s.compose.Down(r.Context(), name)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	s.renderStackDetail(w, r, name, string(out), errMsg)
}

func (s *Server) handleStackDelete(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	// Best-effort teardown before removing the saved definition; a stack
	// that was never deployed simply has nothing to tear down.
	_, _ = s.compose.Down(r.Context(), name)

	if err := s.store.DeleteStack(r.Context(), name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// The list page removes just this row via an htmx DELETE (empty 200
	// body). The detail page instead posts a plain form, since there's
	// nothing left on that page once its stack is gone; a normal redirect
	// sends the browser back to the list.
	if r.Method == http.MethodPost {
		http.Redirect(w, r, "/stacks", http.StatusSeeOther)
		return
	}
	w.WriteHeader(http.StatusOK)
}
