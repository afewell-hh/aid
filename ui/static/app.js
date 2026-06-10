const _M0FP25aidui3src12console__log = (m) => console.log(m);
const _M0FP25aidui3src9set__html = (id, html) => { const e = document.getElementById(id); if (e) e.innerHTML = html; };
const _M0FP25aidui3src10fetch__get = (url, cb) => { fetch(url).then(r => r.text()).then(cb); };
const _M0FP25aidui3src11fetch__post = (url, body, cb) => { fetch(url, { method: "POST", headers: { "Content-Type": "application/json" }, body }).then(r => r.text()).then(cb); };
const _M0FP25aidui3src9api__base = "/api";
function _M0FP25aidui3src16plan__list__html(plans_json) {
  return "";
}
function _M0FP25aidui3src18plan__detail__html(detail_json) {
  return "";
}
function _M0FP25aidui3src19calc__summary__html(calc_json) {
  return "";
}
function _M0FP25aidui3src9bom__html(bom_json) {
  return "";
}
function _M0FP25aidui3src18render__plan__list(target, plans_json) {
  _M0FP25aidui3src9set__html(target, _M0FP25aidui3src16plan__list__html(plans_json));
}
function _M0FP25aidui3src20render__plan__detail(target, detail_json) {
  _M0FP25aidui3src9set__html(target, _M0FP25aidui3src18plan__detail__html(detail_json));
}
function _M0FP25aidui3src11render__bom(target, bom_json) {
  _M0FP25aidui3src9set__html(target, _M0FP25aidui3src9bom__html(bom_json));
}
function _M0FP25aidui3src8api__get(path, cb) {
  _M0FP25aidui3src10fetch__get(`${_M0FP25aidui3src9api__base}${path}`, cb);
}
function _M0FP25aidui3src11load__plans(target) {
  _M0FP25aidui3src8api__get("/plans", (body) => {
    _M0FP25aidui3src9set__html(target, _M0FP25aidui3src16plan__list__html(body));
  });
}
function _M0FP25aidui3src9api__post(path, body, cb) {
  _M0FP25aidui3src11fetch__post(`${_M0FP25aidui3src9api__base}${path}`, body, cb);
}
function _M0FP25aidui3src13trigger__calc(target, plan_id) {
  _M0FP25aidui3src9api__post(`/plans/${plan_id}/calc`, "{}", (body) => {
    _M0FP25aidui3src9set__html(target, _M0FP25aidui3src19calc__summary__html(body));
  });
}
function _M0FP25aidui3src16download__wiring(plan_id, fabric) {
  `${_M0FP25aidui3src9api__base}/plans/${plan_id}/wiring/${fabric}`;
}
function _M0FP25aidui3src11main__entry() {
  _M0FP25aidui3src12console__log("AID UI starting");
}
export { _M0FP25aidui3src18render__plan__list as render_plan_list, _M0FP25aidui3src20render__plan__detail as render_plan_detail, _M0FP25aidui3src11render__bom as render_bom, _M0FP25aidui3src11load__plans as load_plans, _M0FP25aidui3src13trigger__calc as trigger_calc, _M0FP25aidui3src16download__wiring as download_wiring, _M0FP25aidui3src11main__entry as main_entry }
