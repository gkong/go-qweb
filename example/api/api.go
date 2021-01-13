// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// declarations of request and response structs for rest api
//
// due to an ffjson constraint, this can't be in package main.
// to re-generate: ffjson <file>.go

//go:generate ffjson api.go

package api

// ffjson: noencoder
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// ffjson: nodecoder
type TokenResponse struct {
	Token          string `json:"token" binding:"required"`
	TimeToLiveSecs int    `json:"ttl" binding:"required"`
}

// ffjson: nodecoder
type TTLResponse struct {
	TimeToLiveSecs int `json:"ttl" binding:"required"`
}

// ffjson: noencoder
type UpdateRequest struct {
	Note string `json:"note" binding:"required"`
}

// ffjson: nodecoder
type FetchResponse struct {
	Username string `json:"username" binding:"required"`
	Note     string `json:"note" binding:"required"`
}

// ffjson: noencoder
type AddpwdbRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Userid   int    `json:"userid" binding:"required"`
}
