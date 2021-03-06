// The MIT License
//
// Copyright (c) 2020 Temporal Technologies Inc.  All rights reserved.
//
// Copyright (c) 2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package authorization

import (
	"context"
)

type (
	defaultAuthorizer struct {
	}
)

var _ Authorizer = (*defaultAuthorizer)(nil)

// NewDefaultAuthorizer creates a default authorizer
func NewDefaultAuthorizer() Authorizer {
	return &defaultAuthorizer{}
}

func (a *defaultAuthorizer) Authorize(_ context.Context, claims *Claims, target *CallTarget) (Result, error) {

	// TODO: This is a temporary workaround to allow calls to system namespace and
	// calls with no namespace to pass through. When handling of mTLS data is added,
	// we should remove "temporal-system" from here. Handling of call with
	// no namespace will need to be performed at the API level, so that data would
	// be filtered based of caller's permissions to namespaces and system.
	if target.Namespace == "temporal-system" || target.Namespace == "" {
		return Result{Decision: DecisionAllow}, nil
	}
	if claims == nil {
		return Result{Decision: DecisionDeny}, nil
	}
	// Check system level permissions
	if claims.System == RoleAdmin || claims.System == RoleWriter {
		return Result{Decision: DecisionAllow}, nil
	}
	roles, found := claims.Namespaces[target.Namespace]
	if !found || roles == RoleUndefined {
		return Result{Decision: DecisionDeny}, nil
	}

	return Result{Decision: DecisionAllow}, nil
}
