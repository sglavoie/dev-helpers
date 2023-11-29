package server

import (
	"net/http"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
)

type HandlerWithConfig func(w http.ResponseWriter, r *http.Request, cfg models.Config)
