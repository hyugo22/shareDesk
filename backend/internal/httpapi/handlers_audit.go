package httpapi

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/hyugo22/sharedesk/backend/internal/repository"
)

func (s *Server) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := repository.AuditFilter{
		ActorUserID: q.Get("actor_user_id"),
		Action:      q.Get("action"),
	}
	if limit, err := strconv.Atoi(q.Get("limit")); err == nil {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(q.Get("offset")); err == nil {
		filter.Offset = offset
	}
	if from, err := time.Parse(time.RFC3339, q.Get("from")); err == nil {
		filter.From = &from
	}
	if to, err := time.Parse(time.RFC3339, q.Get("to")); err == nil {
		filter.To = &to
	}

	logs, err := s.repos.Audit.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	if q.Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="audit-logs.csv"`)
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"id", "occurred_at", "actor_type", "actor_name", "actor_user_id", "action", "target_type", "target_id", "ip_address"})
		for _, l := range logs {
			actorUserID := ""
			if l.ActorUserID != nil {
				actorUserID = *l.ActorUserID
			}
			_ = cw.Write([]string{
				fmt.Sprintf("%d", l.ID), l.OccurredAt.Format(time.RFC3339), l.ActorType, l.ActorName,
				actorUserID, l.Action, l.TargetType, l.TargetID, l.IPAddress,
			})
		}
		cw.Flush()
		return
	}

	writeJSON(w, http.StatusOK, logs)
}
