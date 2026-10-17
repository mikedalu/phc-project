package service

import "github.com/mikedalu/phc-records/api/internal/domain"

// The workflow state machine.
//
// ═══ ONE MAP, TWO AUDIENCES ═══
//
// This is the authority for the API, and the client renders the buttons from the
// same rules. If the client had its own copy, a stage the server refuses would
// still be offered as a button, and a clerk would learn to distrust the screen.
//
// The transitions are deliberately narrow. "Any stage to any stage" is not a
// state machine, it is a comment.
var allowedTransitions = map[string][]string{
	domain.StageRegistered:     {domain.StageVitalsDone},
	domain.StageVitalsDone:     {domain.StageAwaitingDoctor},
	domain.StageAwaitingDoctor: {domain.StageInConsultation},
	domain.StageInConsultation: {
		domain.StageAwaitingLab, domain.StageAwaitingDrugPay,
		domain.StageAwaitingPharm, domain.StageAdmitted, domain.StageClosed,
	},
	domain.StageAwaitingLab:     {domain.StageAwaitingLabPay, domain.StageLabDone},
	domain.StageAwaitingLabPay:  {domain.StageAwaitingLab},
	domain.StageLabDone:         {domain.StageAwaitingDoctor},
	domain.StageAwaitingDrugPay: {domain.StageAwaitingPharm},
	domain.StageAwaitingPharm:   {domain.StagePharmacyDone},
	domain.StagePharmacyDone:    {domain.StageClosed},
	domain.StageAdmitted:        {domain.StageClosed},
}

// CanTransition reports whether `to` is reachable from `from`.
func CanTransition(from, to string) bool {
	for _, candidate := range allowedTransitions[from] {
		if candidate == to {
			return true
		}
	}
	return false
}

// NextStages lists what the workflow permits from here. The UI renders exactly
// this, so a button can never offer an illegal move.
func NextStages(from string) []string {
	out := make([]string, 0, 4)
	for _, stage := range allowedTransitions[from] {
		out = append(out, stage)
	}
	return out
}

// stageRoles names who may perform a transition INTO a stage.
//
// Coarse on purpose: the fine-grained rule ("only the attending doctor") is a
// service check with data we do not have yet. This map stops the obvious
// mistakes — a cashier moving a patient into consultation — and the service
// still verifies the actor is active and holds a clinical role.
var stageRoles = map[string][]domain.Role{
	domain.StageVitalsDone:      {domain.RoleNurse, domain.RoleDoctor, domain.RoleAdmin},
	domain.StageAwaitingDoctor:  {domain.RoleNurse, domain.RoleFrontDesk, domain.RoleAdmin},
	domain.StageInConsultation:  {domain.RoleDoctor, domain.RoleAdmin},
	domain.StageAwaitingLab:     {domain.RoleDoctor, domain.RoleAdmin},
	domain.StageAwaitingLabPay:  {domain.RoleNurse, domain.RoleDoctor, domain.RoleCashier, domain.RoleAdmin},
	domain.StageLabDone:         {domain.RoleLabScientist, domain.RoleAdmin},
	domain.StageAwaitingDrugPay: {domain.RoleDoctor, domain.RoleAdmin},
	domain.StageAwaitingPharm:   {domain.RoleCashier, domain.RoleAdmin},
	domain.StagePharmacyDone:    {domain.RolePharmacist, domain.RoleAdmin},
	domain.StageAdmitted:        {domain.RoleDoctor, domain.RoleAdmin},
	domain.StageClosed:          {domain.RoleDoctor, domain.RoleNurse, domain.RoleAdmin},
}

// StageAllowedForAnyRole reports whether ANY of the user's roles may move a
// patient into a stage.
//
// This is the function the service must use. Checking only PrimaryRole() is a
// real bug: a nurse who also holds `doctor` has PrimaryRole doctor, and a stage
// allowed for nurses but not doctors would be refused for no reason.
func StageAllowedForAnyRole(stage string, roles []domain.Role) bool {
	for _, role := range roles {
		if StageAllowedForRole(stage, role) {
			return true
		}
	}
	return false
}

// StageAllowedForRole reports whether this role may move a patient INTO a stage.
func StageAllowedForRole(stage string, role domain.Role) bool {
	allowed, known := stageRoles[stage]
	if !known {
		return false
	}
	for _, candidate := range allowed {
		if candidate == role {
			return true
		}
	}
	return false
}
