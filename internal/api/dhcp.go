package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/tchevalleraud/EasyNetServices/internal/dhcp"
)

// writeDHCPError maps the errors of the DHCP manager to API errors.
func (s *Server) writeDHCPError(w http.ResponseWriter, r *http.Request, err error) {
	var rejected *dhcp.RejectedError
	switch {
	case errors.Is(err, dhcp.ErrSubnetNotFound):
		writeError(w, http.StatusNotFound, "not_found", "subnet not found")
	case errors.Is(err, dhcp.ErrLeaseNotFound):
		writeError(w, http.StatusNotFound, "not_found", "lease not found")
	case errors.Is(err, dhcp.ErrNotRunning):
		writeError(w, http.StatusServiceUnavailable, "dhcp_unavailable", "the DHCP server is not running")
	case errors.Is(err, dhcp.ErrApply):
		writeError(w, http.StatusInternalServerError, "apply_failed", err.Error())
	case errors.As(err, &rejected):
		writeError(w, http.StatusBadRequest, "config_rejected", rejected.Message)
	default:
		s.writeUpdateError(w, r, err)
	}
}

// pathFamily returns the family of the request path, writing a 404 when it is unknown.
func pathFamily(w http.ResponseWriter, r *http.Request) (dhcp.Family, bool) {
	f, ok := dhcp.ParseFamily(r.PathValue("family"))
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "endpoint not found")
	}
	return f, ok
}

// pathSubnetID returns the subnet ID of the request path, writing a 404 when it is invalid.
func pathSubnetID(w http.ResponseWriter, r *http.Request) (uint32, bool) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if err != nil || id == 0 {
		writeError(w, http.StatusNotFound, "not_found", "subnet not found")
		return 0, false
	}
	return uint32(id), true
}

func (s *Server) handleGetDHCPSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.dhcp.Settings(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handlePutDHCPSettings(w http.ResponseWriter, r *http.Request) {
	var settings dhcp.Settings
	if !decodeJSON(w, r, &settings) {
		return
	}
	saved, err := s.dhcp.UpdateSettings(r.Context(), settings)
	if err != nil {
		s.writeDHCPError(w, r, err)
		return
	}
	s.log.Info("DHCP settings updated", "username", userFrom(r.Context()).Username)
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleGetDHCPStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.dhcp.Status(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleListDHCPInterfaces(w http.ResponseWriter, r *http.Request) {
	ifaces, err := s.dhcp.Interfaces()
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, ifaces)
}

func (s *Server) handleListSubnets(w http.ResponseWriter, r *http.Request) {
	f, ok := pathFamily(w, r)
	if !ok {
		return
	}
	subnets, err := s.dhcp.Subnets(r.Context(), f)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, subnets)
}

func (s *Server) handleCreateSubnet(w http.ResponseWriter, r *http.Request) {
	f, ok := pathFamily(w, r)
	if !ok {
		return
	}
	var subnet dhcp.Subnet
	if !decodeJSON(w, r, &subnet) {
		return
	}
	saved, err := s.dhcp.CreateSubnet(r.Context(), f, subnet)
	if err != nil {
		s.writeDHCPError(w, r, err)
		return
	}
	s.log.Info("DHCP subnet created", "subnet", saved.Subnet, "username", userFrom(r.Context()).Username)
	writeJSON(w, http.StatusCreated, saved)
}

func (s *Server) handleGetSubnet(w http.ResponseWriter, r *http.Request) {
	f, ok := pathFamily(w, r)
	if !ok {
		return
	}
	id, ok := pathSubnetID(w, r)
	if !ok {
		return
	}
	subnet, err := s.dhcp.Subnet(r.Context(), f, id)
	if err != nil {
		s.writeDHCPError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, subnet)
}

func (s *Server) handlePutSubnet(w http.ResponseWriter, r *http.Request) {
	f, ok := pathFamily(w, r)
	if !ok {
		return
	}
	id, ok := pathSubnetID(w, r)
	if !ok {
		return
	}
	var subnet dhcp.Subnet
	if !decodeJSON(w, r, &subnet) {
		return
	}
	saved, err := s.dhcp.UpdateSubnet(r.Context(), f, id, subnet)
	if err != nil {
		s.writeDHCPError(w, r, err)
		return
	}
	s.log.Info("DHCP subnet updated", "subnet", saved.Subnet, "username", userFrom(r.Context()).Username)
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleDeleteSubnet(w http.ResponseWriter, r *http.Request) {
	f, ok := pathFamily(w, r)
	if !ok {
		return
	}
	id, ok := pathSubnetID(w, r)
	if !ok {
		return
	}
	if err := s.dhcp.DeleteSubnet(r.Context(), f, id); err != nil {
		s.writeDHCPError(w, r, err)
		return
	}
	s.log.Info("DHCP subnet deleted", "id", id, "username", userFrom(r.Context()).Username)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListLeases(w http.ResponseWriter, r *http.Request) {
	f, ok := pathFamily(w, r)
	if !ok {
		return
	}
	var subnetID uint32
	if v := r.URL.Query().Get("subnet"); v != "" {
		id, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_parameter", "subnet must be a subnet ID")
			return
		}
		subnetID = uint32(id)
	}
	leases, err := s.dhcp.Leases(r.Context(), f, subnetID)
	if err != nil {
		s.writeDHCPError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, leases)
}

func (s *Server) handleDeleteLease(w http.ResponseWriter, r *http.Request) {
	f, ok := pathFamily(w, r)
	if !ok {
		return
	}
	address := r.PathValue("address")
	if err := s.dhcp.DeleteLease(r.Context(), f, address); err != nil {
		s.writeDHCPError(w, r, err)
		return
	}
	s.log.Info("DHCP lease deleted", "address", address, "username", userFrom(r.Context()).Username)
	w.WriteHeader(http.StatusNoContent)
}
