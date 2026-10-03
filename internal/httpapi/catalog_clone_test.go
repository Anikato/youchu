package httpapi

import (
	"fmt"
	"net/http"
	"testing"
)

func TestLocationClone(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	kitchen := mustCreate(t, h, cookie, `{"name":"厨房","type":"area"}`)
	drawer := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"抽屉","type":"fixed","code":"K101","parent_id":%d}`, kitchen.ID))
	box := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"零件盒","type":"movable","code":"B101","parent_id":%d,"icon":"box"}`, drawer.ID))
	mustItem(t, h, cookie, fmt.Sprintf(`{"name":"螺丝","locations":[{"location_id":%d}]}`, box.ID))

	rec := postLoc(h, cookie, locURL(box.ID)+"/clone", `{}`)
	clone := assertCreated(t, rec)
	if clone.Name != "零件盒" || clone.Type != "movable" || clone.Code == nil || *clone.Code != "B102" {
		t.Fatalf("clone=%+v", clone)
	}
	if clone.ParentID == nil || *clone.ParentID != drawer.ID {
		t.Fatalf("parent=%v want %d", clone.ParentID, drawer.ID)
	}
	if clone.Icon == nil || *clone.Icon != "box" {
		t.Fatalf("icon=%v", clone.Icon)
	}
	if clone.DirectItemCount != 0 {
		t.Fatalf("clone items=%d", clone.DirectItemCount)
	}
	src := requireLoc(t, h, cookie, box.ID)
	if src.Version != 1 || src.DirectItemCount != 1 {
		t.Fatalf("source=%+v", src)
	}

	rec = postLoc(h, cookie, locURL(box.ID)+"/clone", `{}`)
	second := assertCreated(t, rec)
	if second.Code == nil || *second.Code != "B103" {
		t.Fatalf("skip taken=%+v", second)
	}

	rec = postLoc(h, cookie, locURL(kitchen.ID)+"/clone", `{}`)
	assertInvalidFields(t, rec, map[string]string{"type": "只有移动容器可以克隆"})
	rec = postLoc(h, cookie, locURL(drawer.ID)+"/clone", `{}`)
	assertInvalidFields(t, rec, map[string]string{"type": "只有移动容器可以克隆"})

	rec = request(h, http.MethodPost, locURL(box.ID)+"/clone", `{}`, "http://evil.example", testRemote, cookie)
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	rec = request(h, http.MethodPost, locURL(box.ID)+"/clone", `{}`, webOrigin, testRemote, "")
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")
	rec = request(h, http.MethodPost, "/api/v1/locations/abc/clone", `{}`, "http://evil.example", testRemote, "")
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	rec = postLoc(h, cookie, locURL(box.ID)+"/clone?x=1", `{}`)
	assertInvalidFields(t, rec, map[string]string{"x": "不支持的参数"})
	rec = postLoc(h, cookie, "/api/v1/locations/999999/clone", `{}`)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = postLoc(h, cookie, locURL(box.ID)+"/clone", `[`)
	assertError(t, rec, http.StatusBadRequest, "invalid_body", "请求格式不正确")
	rec = request(h, http.MethodGet, locURL(box.ID)+"/clone", "", "", testRemote, cookie)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
}
