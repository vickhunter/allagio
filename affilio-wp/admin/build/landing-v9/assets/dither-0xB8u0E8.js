import{d as e,ft as t,o as n,pt as r}from"./Page-G2RQS0W1.js";var i=r(t(),1),a=e(),o={11:`goQ2w/RYi3UkakR4a9LzHhmb912vxHS0W8EmReyg1FPw6dEzPty6eq3tRhhsn5wwJQ/HqYkhpH00Uk9wl74rzSCrAR0NShE5ecAvCJPW+xLMFLxQP0cDgKLq+oZ/ss+ot2XLI7U8hZhk/arnWusiF5UVYQm45tAWkqcGYDoM2Zot2qVmSeHJj6Yn/4H1ux9yoe5uSP6s43v4OPEOQG/vvSlpxQteTQdV5N2DSxqjrsYEnXOH/OhxHAUsX9hBMc62bZHIji5DuT3lyvZXiLOWCjJCjOAbKmgA8mKemQK/wlHX4n4odxNj1ds7VpBcsUxZlHY3NflU342w0958imcQTsii5sFMpFe/YiffsL6PyHKQEI7QtM1/4RzfXxeedG3ZI51qh38ig9wsAxF1+Av9iR9rNbaKSUHdw29nvPgUKVyBEg44Yr2cBwtz3+ld4u1ovl0nkxapzulyHube95esXy9KuFOwgHL3Knwp731VK5dmIAhklYTm6O/MeoZlZgvMPwd3t29dqCoDl4fdasgFB5k6ESFgVpDiXS+TAWJRYw2Nn7/0WqdxN+O+8NHqIQQzaa9wg8ISrQw/uK5nS9NGCD8Fzx/4Nk6VT2EbYwY56EhFreFLYLbKnS4uwbTlx2si47GTrmJYk3u179hjX+xlzBhzhM7gJ52gJ9WN9TCGUqQ=`,5:`rE8+VuhkVChfFxEVsyKSA2+kmmc5PZ3r9roEobFdGv1CegfFasZyWFqTFpErL+RXLhQMUTx9oOBg86OAlhvS/KdJXu+5Y7xGTf7/HcmE3sPljh47nIhwMSwnRPHRJgE2Zcr5IOJmHxk1YjffAm4F2NNBE2F3qnE6m+oAr3X3i/toKtvaCa7BtqtLe1ui1UCYnykQid1SzMuyGIby3M+38EyN7EXImdSmnhxzg2wLUIwjDZU45nlpqePXsILHBhK7lHTW6Vm4XErC4Yf0zb4kSC34R+eXMw+K+jRrfq1T0A542SFDML8IvU5tf1WBkKW0tc4yxIWPCu18dj/AJfXuqD/cKrBAETGtgIx0Rn3rM7tAMVMY710tAA83a5n64z6c6xraZcdSoIEbwtJ7Y/Jf4GtfXA9HOxCKcQYo6iJf8h1pzAUR7a5Dej/k8hKGL2e9tjBVbGOuyVZxEV7xgI6bkc1CUeJ/9SPIeXnhIYNAV9BmyAP/9K0ypzFfCRIoLtY+0gIPEO7QBO46Q5U0+V2Skki7/ZgVzqp9p0NNgxGWL0Hllj6y/uXMVZWVF7GFhbwUNvxMQRJQjwpKQSfK9elwEVmGYbq95Xg96OEgNqjR9QUwbIzGjoQ9ppXxMCBaU3QstNIC6BdOfGw3utY6KoQm8NnVaclkdRsKqzH0kJPkTAM=`,23:`Cuf5FSbQduNd7C30tQF7pbrksFfx0+08nlsq/SmGJGXrbIq/1mjOrWdpRtR/rJIjZvrFmHDoWT0nAr5Dp2D8wa8sl/9VEVq3R+G9/sL4ekm4SFz3UvbgEowPGNziS3Qbh8oDjRcWOdiIw1NKoICxnz9EnDIHlY9rHhkFtscumTe00mTbc6b7MwCBs6vPnXKL119eIY5iVhzR80JMTc2Jy6MUKHyb3dqoaqKQqQmCVFHlcZMabaQIWFD1b6odfmGuMeY7Yw1OdZqWMKG5xsw6331PEAZ3JYVAg8gMlNlB7tW7wMlued41DiITBCsLPjhFxPIfeDbq77KE6fC8IJEvNCpgjfhG28hekwf6fifK3rz7R41wo0iud2KyRVT+Y1b27+sZzTal2r4PmCzTOjVbjeYu1mv9ojiCvdR3YLwatGWxFkBKTvfaFpPyzw29x+aUby+YR0mHE3xAdjC+3LHk4pbAJI73vvdQJhAFl7Qs80g8tAd3q487zo2SY/kPQbZOwL5p17Gsi5Hr1D/jm3mJi3v1wBtIXvqPoPTTPEGZgQSK8K4jsy34i+PF94R+EKw9Q5apIIgCCpGzbKDViuJ50aWhfcmi3NiqXPGeRtM6A+M+W3obgGEKXK3sm/WdZJ+f0HrON05uQk5Zwng7Fdy0rhUHUgg1+XK/6H7Gl3TIMT8=`},s={horizon:{field:`
float field(vec2 uv, float t) {
  vec2 p = uv * uSize;
  vec2 size = uSize;
  float r = max(min(size.x * 0.42, size.y * 0.8), 90.0);
  vec2 d = (p - vec2(0.5 * size.x, -0.15 * r)) / vec2(1.25 * r, r);
  float wobble = fbm(vec2(atan(d.y, d.x) * 1.8, t * 0.04)) - 0.5;
  float rim = length(d) * (1.0 + 0.14 * wobble);
  float sun = exp(-3.4 * max(rim - 0.42, 0.0)) * 0.78;
  float h = p.y / r;
  float drift = (fbm(vec2(p.x / 320.0, t * 0.03)) - 0.5) * 2.4;
  float band = 0.5 + 0.5 * sin(log(1.0 + 7.0 * h) * 8.0 - t * 0.45 + drift);
  float haze = smoothstep(0.25, 0.95, band) * exp(-1.9 * h) * 0.5;
  float ground = exp(-p.y / 22.0) * 0.32;
  float sides = smoothstep(0.0, 0.2, uv.x) * smoothstep(1.0, 0.8, uv.x);
  return (sun * (1.0 + haze) + haze * 0.35 + ground) * sides * (1.0 + 0.04 * sin(t * 0.4));
}`},glow:{field:`
float field(vec2 uv, float t) {
  vec2 p = uv * uSize;
  vec2 c = 0.5 * uSize + vec2(sin(t * 0.21), cos(t * 0.17)) * 0.05 * uSize;
  float r = 0.5 * min(uSize.x, uSize.y);
  float d = length(p - c) / r;
  float n = fbm(p / 170.0 + vec2(t * 0.05, -t * 0.04));
  return exp(-1.7 * d * d) * (0.75 + 0.45 * n) + 0.12 * n * exp(-d);
}`},aurora:{field:`
float field(vec2 uv, float t) {
  vec2 p = uv * uSize;
  float sway = fbm(vec2(p.x / 600.0, t * 0.05));
  float d = uv.y - (0.45 + 0.35 * (sway - 0.5));
  float curtain = d > 0.0 ? exp(-d * 4.0) : exp(d * 22.0);
  float rays = fbm(vec2(p.x / 70.0 + sway * 6.0, p.y / 900.0 - t * 0.08));
  float shimmer = fbm(vec2(p.x / 40.0 - t * 0.05, 3.0));
  return curtain * (0.25 + 0.75 * smoothstep(0.3, 0.75, rays)) * (0.6 + 0.5 * shimmer);
}`},waves:{field:`
float field(vec2 uv, float t) {
  vec2 q = uv * uSize / 32.0;
  float warp = fbm(q * 0.09 + vec2(t * 0.035, t * 0.02));
  float s = 0.5 + 0.5 * sin(q.x * 0.9 + q.y * 0.6 + warp * 5.0 - t * 0.4);
  float crest = s * s * s * s;
  return crest * (0.5 + 0.5 * uv.y) + 0.12 * warp;
}`},clouds:{field:`
float field(vec2 uv, float t) {
  vec2 p = uv * uSize;
  float n = fbm(p / 210.0 + vec2(t * 0.03, t * 0.012));
  float m = fbm(p / 80.0 - vec2(t * 0.02, 0.0));
  return smoothstep(0.28, 0.78, n) * (0.7 + 0.5 * m);
}`},sunset:{palette:[`#1d2021`,`#3c3836`,`#b16286`,`#d65d0e`,`#fabd2f`],matrix:8,field:`
float field(vec2 uv, float t) {
  float rise = 4.0 * (0.5 - 0.5 * cos(6.2831853 * t / 90.0));
  vec2 p = vec2(uv.x, 1.0 - uv.y) * uSize;
  float x = p.x, y = p.y, w = uSize.x, h = uSize.y;
  float hy = h * 0.80;
  float r = w * 0.30;
  float cx = w * 0.5;
  float cy = hy - r * (0.42 - 0.10 * rise);
  float sky = 2.95 * pow(ss(hy * 0.30, hy, y), 2.3);
  float d = length(vec2(x - cx, y - cy));
  sky += 0.6 * exp(-max(d - r, 0.0) / (r * 0.8)) * (y < hy ? 1.0 : 0.0);
  sky = min(sky, 2.9);
  float sun = 4.4 - 1.6 * ss(cy - r, cy + r * 0.6, y);
  float rel = (y - cy) / r;
  bool gap = false;
  float starts[5] = float[5](0.02, 0.14, 0.24, 0.32, 0.38);
  for (int i = 0; i < 5; i++) {
    gap = gap || (rel > starts[i] && rel < starts[i] + 0.025 + 0.012 * float(i));
  }
  bool inside = d < r && !gap && y < hy;
  float f = inside ? sun : ((d < r && gap) ? sky - 1.3 : sky);
  if (y < hy) return f / 4.0;
  float ground = 1.15 * (1.0 - ss(hy, hy + (h - hy) * 0.9, y));
  float down = (y - hy) / (h - hy);
  float ry = (y - hy) / 4.0;
  bool reflection = abs(x - cx) < r * (1.0 - down * 0.6)
    && sin(ry * 0.9 + pow(max(ry, 0.0), 1.3) * 0.12) > 0.55;
  if (reflection && y < hy + (h - hy) * 0.7) ground += 1.0;
  return ground / 4.0;
}`},planet:{palette:[`#2e3440`,`#3b4252`,`#434c5e`,`#5e81ac`,`#88c0d0`],matrix:8,field:`
float field(vec2 uv, float t) {
  float rise = 4.0 * (0.5 - 0.5 * cos(6.2831853 * t / 90.0));
  vec2 p = vec2(uv.x, 1.0 - uv.y) * uSize;
  float x = p.x, y = p.y, w = uSize.x, h = uSize.y;
  float R = w * 1.25;
  float cx = w * 0.62;
  float cy = h + R * 0.72 - rise * h * 0.012;
  float d = length(vec2(x - cx, y - cy));
  float edge = d - R;
  float sky = 1.25 * pow(ss(h * 0.18, h, y), 2.4);
  sky += 1.9 * exp(-max(edge, 0.0) / (h * 0.10));
  float ang = atan(y - cy, x - cx);
  float light = 0.55 + 0.45 * cos(ang - (-1.5707963 - 0.35));
  float limb = 4.6 * exp(-max(-edge, 0.0) / (h * 0.018));
  float body = 0.1 + limb * light + 0.5 * ss(R * 0.96, R, d) * light;
  return (edge < 0.0 ? min(body, 4.6) : min(sky, 3.2)) / 4.0;
}`},moonrise:{palette:[`#16161d`,`#223249`,`#2d4f67`,`#7e9cd8`,`#dcd7ba`],matrix:8,field:`
float field(vec2 uv, float t) {
  float drift = 3.0 * (0.5 - 0.5 * cos(6.2831853 * t / 90.0));
  float shimmer = t * 0.4 * 1.6;
  vec2 p = vec2(uv.x, 1.0 - uv.y) * uSize;
  float x = p.x, y = p.y, w = uSize.x, h = uSize.y;
  float hy = h * 0.84;
  float r = w * 0.15;
  float cx = w * 0.58 + drift * r * 0.25;
  float cy = h * 0.52;
  float sky = 1.75 * pow(ss(h * 0.2, hy, y), 2.4);
  float d = length(vec2(x - cx, y - cy));
  sky = min(sky + 1.0 * exp(-max(d - r, 0.0) / (r * 1.5)), 2.5);
  bool shadow = length(vec2(x - (cx - r * 0.42), y - (cy - r * 0.22))) < r * 0.92;
  bool lit = d < r && !shadow;
  float f = lit ? 4.2 : (d < r ? min(sky, 1.2) + 0.25 : sky);
  if (y < hy) return f / 4.0;
  float depth = (y - hy) / (h - hy);
  float sea = 1.6 - 1.3 * ss(0.0, 0.85, depth);
  float width = r * (0.55 + depth * 0.6);
  float wob = sin(depth * 22.0 + shimmer);
  bool column = abs(x - cx) < width * (1.0 - 0.35 * wob * wob);
  bool dash = sin((y - hy) / 4.0 * 0.7 + shimmer + sin(x / 4.0 * 0.05) * 0.6) > 0.15;
  if (column && dash) sea += 1.6 * (1.0 - depth * 0.7);
  return sea / 4.0;
}`},orbit:{palette:[`#2e3440`,`#3b4252`,`#88c0d0`],matrix:4,field:`
float field(vec2 uv, float t) {
  vec2 p = vec2(uv.x, 1.0 - uv.y) * uSize;
  float x = p.x, y = p.y, w = uSize.x, h = uSize.y;
  float cx = w * 0.5, cy = h * 0.98, s = w * 1.6;
  float r = w * 0.62 * (1.0 + 0.04 * sin(6.2831853 * t / 16.0));
  float d = length(vec2(x - cx, y - cy));
  float ring = exp(-pow((d - r) / (0.035 * s), 2.0));
  float halo = exp(-pow((d - r) / (0.22 * s), 2.0));
  float f = 0.9 * halo + 1.8 * ring;
  return f * (0.6 + 0.4 * ss(h * 0.55, h * 0.92, y)) / 2.0;
}`},meridian:{palette:[`#272e33`,`#374145`,`#a7c080`],matrix:4,field:`
float field(vec2 uv, float t) {
  vec2 p = vec2(uv.x, 1.0 - uv.y) * uSize;
  float x = p.x, y = p.y, w = uSize.x, h = uSize.y;
  float u = x * 0.866 + y * 0.5;
  float unit = w * 0.16;
  vec2 ref = vec2(w * 0.40, h * 0.86);
  float c0 = ref.x * 0.866 + ref.y * 0.5 + unit * 0.6 * sin(6.2831853 * t / 20.0);
  float f = 0.0;
  float amps[3] = float[3](1.0, 0.72, 0.46);
  for (int k = 0; k < 3; k++) {
    float c = c0 + float(k) * unit * 1.6;
    float e = abs(u - c);
    f = max(f, amps[k] * (1.0 - ss(unit * 0.42, unit * 0.52, e)));
  }
  return f * ss(h * 0.55, h * 1.02, y);
}`},eclipse:{palette:[`#191724`,`#26233a`,`#c4a7e7`,`#ebbcba`],matrix:4,field:`
float field(vec2 uv, float t) {
  vec2 p = vec2(uv.x, 1.0 - uv.y) * uSize;
  float x = p.x, y = p.y, w = uSize.x, h = uSize.y;
  float cx = w * 0.62, cy = h * 0.86, r = w * 0.36;
  float ox = r * (0.20 + 0.10 * sin(6.2831853 * t / 24.0));
  float oy = -r * 0.12;
  float d1 = length(vec2(x - cx, y - cy));
  float d2 = length(vec2(x - cx - ox, y - cy - oy));
  float disc = 1.0 - ss(r * 0.96, r * 1.04, d1);
  float shadow = ss(r * 0.80, r * 1.10, d2);
  float lit = disc * shadow;
  float grad = ss(cy + r, cy - r, y);
  float glow = exp(-pow(max(d1 - r, 0.0) / (r * 0.55), 2.0)) * (1.0 - disc);
  float f = lit * (1.6 + 1.4 * grad) + 0.9 * glow + disc * 0.6;
  return min(f, 1.0 + 2.0 * ss(h * 0.6, h * 0.75, y)) / 3.0;
}`},ridgeline:{palette:[`#1e2326`,`#272e33`,`#2e383c`,`#374145`,`#4f585e`,`#7a8478`],matrix:4,seed:11,field:`
vec4 ridgeShape(int i) {
  return i == 0 ? vec4(0.76, 0.06, 2.0, 3.4) : (i == 1 ? vec4(0.82, 0.055, 2.6, 2.6) : (i == 2 ? vec4(0.88, 0.05, 3.2, 1.9) : vec4(0.945, 0.04, 3.8, 1.2)));
}
float ridgeAt(float x, float k, int i) {
  vec4 L = ridgeShape(i);
  float fi = float(i);
  float drift = k * 0.02 * (fi + 1.0);
  float r = fbmN(vec2((x + drift) * L.z + 7.7 * fi, 3.1 * fi), 5, 0.55);
  return L.x - L.y * (r - 0.5) * 3.0;
}
float field(vec2 uv, float t) {
  float k = t * 0.1;
  float x = uv.x * uAspect, y = 1.0 - uv.y, W = uAspect;
  float horizon = 0.85;
  float above = max(horizon - y, 0.0);
  float cx = (x - W * 0.5) / max(W, 1.0);
  float glow = ss(0.25, 0.0, above + 0.25 * cx * cx);
  float v = 0.05 + 1.6 * pow(ss(horizon * 0.9, 0.0, above), 2.0) + 1.4 * pow(glow, 3.0);
  float px = 1.0 / uSize.y;
  for (int i = 0; i < 4; i++) {
    vec4 L = ridgeShape(i);
    float ridge = ridgeAt(x, k, i);
    float slope = abs(ridgeAt(x + px, k, i) - ridgeAt(x - px, k, i)) * 0.5;
    float depth = y - ridge;
    float val = L.w - 0.9 * ss(0.0, 0.22, depth);
    bool rim = depth >= 0.0 && depth < 4.0 * max(px, slope * 1.1);
    if (rim) val = 4.6 - 0.9 * float(i);
    if (depth >= 0.0) v = val;
  }
  return v / 5.0;
}`},contour:{palette:[`#2e3440`,`#3b4252`,`#434c5e`,`#4c566a`,`#5e81ac`,`#88c0d0`],matrix:2,seed:5,field:`
float level(vec2 q, float k) {
  float sc = 0.9;
  float wx = fbmN(vec2(q.x * sc + 4.0, q.y * sc), 3, 0.5);
  float wy = fbmN(vec2(q.x * sc, q.y * sc + 9.0), 3, 0.5);
  float e = fbmN(vec2(q.x * 1.1 * sc + wx * 0.8, q.y * 1.1 * sc + wy * 0.8), 4, 0.4);
  e = ss(0.36, 0.80, e);
  e *= 0.15 + 0.85 * ss(0.30, 0.95, q.y);
  return e * 10.0 + k * 0.25;
}
float field(vec2 uv, float t) {
  float k = t * 0.25;
  vec2 q = vec2(uv.x * uAspect, 1.0 - uv.y);
  float px = 1.0 / uSize.y;
  float lv = level(q, k);
  float terrace = floor(lv);
  float frac = lv - terrace;
  float tl = clamp(terrace / 10.0, 0.0, 1.0);
  float base = floor(tl * 22.0) / 4.0;
  float gx = (level(q + vec2(px, 0.0), k) - level(q - vec2(px, 0.0), k)) * 0.5;
  float gy = (level(q + vec2(0.0, px), k) - level(q - vec2(0.0, px), k)) * 0.5;
  float g = length(vec2(gx, gy));
  bool edge = frac < clamp(g * 2.0, 0.02, 0.5);
  if (edge && terrace >= 1.0) base += 1.0;
  return base / 5.0;
}`},dunes:{palette:[`#191724`,`#1f1d2e`,`#26233a`,`#403d52`,`#6e6a86`,`#c4a7e7`],matrix:4,seed:23,field:`
float field(vec2 uv, float t) {
  float k = t * 0.15;
  float x = uv.x * uAspect, y = 1.0 - uv.y;
  float u = x * cos(1.18) + y * sin(1.18);
  float warp = fbmN(vec2(x * 0.8, y * 0.8 + 2.0), 4, 0.5) * 2.4;
  float wave = (u + warp * 0.45) * 7.0 + k * 0.35;
  float f = wave - floor(wave);
  float lit = ss(0.0, 0.78, f) * ss(1.0, 0.80, f);
  float crest = ss(0.6, 0.80, f) * ss(0.86, 0.80, f);
  float amp = ss(0.32, 0.72, fbmN(vec2(x * 0.7 + 30.0, y * 0.7), 3, 0.5));
  amp = (0.35 + 0.65 * amp) * pow(ss(0.18, 1.0, y), 1.5) * 2.0;
  return (0.15 + amp * (2.6 * lit + 2.2 * crest)) / 5.0;
}`},corona:{palette:[`#2e3440`,`#88c0d0`],matrix:8,field:`
float field(vec2 uv, float t) {
  vec2 p = vec2(uv.x, 1.0 - uv.y) * uSize;
  float W = uSize.x, H = uSize.y;
  float cx = W * 0.50, cy = H * 0.79, R = W * 0.36;
  float wd = R * 0.38 * (1.0 + 0.18 * sin(6.2831853 * t / 20.0));
  float d = length(p - vec2(cx, cy)) - R;
  float corona = d > 0.0 ? exp(-d / wd) : 0.0;
  float rim = d <= 0.0 ? exp(d / (R * 0.035)) * 0.9 : 0.0;
  float f = (0.92 * corona + rim) * ss(H * 0.30, H * 0.85, p.y);
  return clamp((clamp(f, 0.0, 1.0) - 0.035) / 0.965, 0.0, 1.0);
}`},orb:{palette:[`#282828`,`#d79921`],matrix:8,field:`
float field(vec2 uv, float t) {
  vec2 p = vec2(uv.x, 1.0 - uv.y) * uSize;
  float W = uSize.x, H = uSize.y;
  float cx = W * 0.66, cy = H * 0.82, R = W * 0.52;
  float a = 6.2831853 * t / 30.0;
  vec3 L = normalize(vec3(-0.55 + 0.25 * cos(a), -0.55 + 0.20 * sin(a), 0.62));
  float dx = (p.x - cx) / R, dy = (p.y - cy) / R;
  float r2 = dx * dx + dy * dy;
  if (r2 >= 1.0) return 0.0;
  float nz = sqrt(clamp(1.0 - r2, 0.0, 1.0));
  float lam = clamp(dx * L.x + dy * L.y + nz * L.z, 0.0, 1.0);
  float f = 0.04 + 0.96 * pow(lam, 1.6);
  float back = clamp(-(dx * L.x + dy * L.y) / sqrt(r2 + 1e-9), 0.0, 1.0);
  float rim = exp(-(1.0 - sqrt(r2)) / 0.05) * back * 0.75;
  f = max(f, rim);
  return clamp((clamp(f, 0.0, 1.0) - 0.035) / 0.965, 0.0, 1.0);
}`},veil:{palette:[`#191724`,`#c4a7e7`],matrix:8,field:`
float field(vec2 uv, float t) {
  vec2 p = vec2(uv.x, 1.0 - uv.y) * uSize;
  float W = uSize.x, H = uSize.y;
  float ang = radians(-70.0);
  float S = max(W, H);
  float along = (p.x * cos(ang) + p.y * sin(ang)) / S;
  vec4 beams[4] = vec4[4](vec4(0.07, 0.045, 0.40, 1.0), vec4(-0.13, 0.090, 0.30, -1.0), vec4(-0.30, 0.035, 0.60, 2.0), vec4(-0.40, 0.070, 0.30, -1.0));
  float acc = 1.0;
  for (int i = 0; i < 4; i++) {
    vec4 b = beams[i];
    float cc = b.x - 0.5 + 0.03 * b.w * sin(6.2831853 * t / 25.0);
    acc *= 1.0 - b.z * exp(-pow((along - cc) / b.y, 2.0));
  }
  float f = 1.0 - acc;
  f *= 0.35 + 0.65 * ss(-0.1, 1.0, p.y / H);
  f *= ss(H * 0.25, H * 0.8, p.y);
  return clamp((clamp(f, 0.0, 1.0) - 0.035) / 0.965, 0.0, 1.0);
}`}},c=`#version 300 es
void main() {
  vec2 corner = vec2(gl_VertexID == 1 ? 3.0 : -1.0, gl_VertexID == 2 ? 3.0 : -1.0);
  gl_Position = vec4(corner, 0.0, 1.0);
}`,l=`
uniform int uMatrix;

float bayer(vec2 cell) {
  vec2 c = floor(cell);
  float sum = 0.0;
  float cells = float(uMatrix * uMatrix);
  float weight = cells * 0.25;
  int levels = uMatrix == 8 ? 3 : (uMatrix == 4 ? 2 : 1);
  for (int i = 0; i < 3; i++) {
    if (i >= levels) break;
    vec2 b = mod(c, 2.0);
    sum += weight * mod(2.0 * b.x + 3.0 * b.y, 4.0);
    c = floor(c * 0.5);
    weight *= 0.25;
  }
  return (sum + 0.5) / cells;
}
`,u=`precision highp float;
precision highp sampler2D;
uniform vec2 uView;
uniform vec2 uSize;
uniform float uAspect;
uniform float uDot;
uniform float uTime;
uniform vec2 uPointer;
uniform float uPointerOn;
uniform sampler2D uNoise;
out vec4 outColor;

float hash(vec2 p) {
  return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453);
}

float noise(vec2 p) {
  vec2 i = floor(p);
  vec2 f = fract(p);
  f = f * f * (3.0 - 2.0 * f);
  float a = hash(i);
  float b = hash(i + vec2(1.0, 0.0));
  float c = hash(i + vec2(0.0, 1.0));
  float d = hash(i + vec2(1.0, 1.0));
  return mix(mix(a, b, f.x), mix(c, d, f.x), f.y);
}

float fbm(vec2 p) {
  float sum = 0.0;
  float amp = 0.5;
  mat2 turn = mat2(0.8, 0.6, -0.6, 0.8);
  for (int i = 0; i < 5; i++) {
    sum += amp * noise(p);
    p = turn * p * 2.0 + 11.7;
    amp *= 0.5;
  }
  return sum;
}

float ss(float a, float b, float x) {
  float t = clamp((x - a) / (b - a), 0.0, 1.0);
  return t * t * (3.0 - 2.0 * t);
}

// Value noise over a 256 x 256 lattice read from a texture.
float hv(ivec2 p) { return texelFetch(uNoise, ivec2(p.x & 255, p.y & 255), 0).r; }
float v2(vec2 p) {
  vec2 i = floor(p);
  vec2 f = p - i;
  vec2 s = f * f * (3.0 - 2.0 * f);
  ivec2 ii = ivec2(i);
  float a = hv(ii);
  float b = hv(ii + ivec2(1, 0));
  float c = hv(ii + ivec2(0, 1));
  float d = hv(ii + ivec2(1, 1));
  return (a + (b - a) * s.x) * (1.0 - s.y) + (c + (d - c) * s.x) * s.y;
}
float fbmN(vec2 p, int octaves, float gain) {
  float total = 0.0;
  float amp = 1.0;
  float norm = 0.0;
  float f = 1.0;
  for (int o = 0; o < 5; o++) {
    if (o >= octaves) break;
    float fo = float(o);
    total += amp * v2(vec2(p.x * f + 17.3 * fo, p.y * f - 9.1 * fo));
    norm += amp;
    amp *= gain;
    f *= 2.03;
  }
  return total / norm;
}
`,d=`
void main() {
  float v = field(gl_FragCoord.xy / uView, uTime);
  outColor = vec4(clamp(v, 0.0, 1.0), 0.0, 0.0, 1.0);
}`,f=`
uniform float uGain;
uniform int uColours;
uniform vec4 uPal[6];
${l}
void main() {
  vec2 uv = gl_FragCoord.xy / uView;
  float f = clamp(field(uv, uTime), 0.0, 1.0) * float(uColours - 1) * uGain;
  vec2 away = (uv - uPointer) * uSize;
  float glow = uPointerOn * exp(-dot(away, away) / 28000.0);
  float threshold = bayer(vec2(floor(gl_FragCoord.x), uView.y - 1.0 - floor(gl_FragCoord.y)));
#ifdef ONE_BIT
  f += glow * 0.55;
  int idx = f > threshold ? 1 : 0;
#else
  f += glow * 1.4;
  int idx = int(clamp(floor(f + threshold), 0.0, float(uColours - 1)));
#endif
  outColor = uPal[idx];
}`,p=`
uniform sampler2D uMedia;
uniform vec2 uMediaSize;

float field(vec2 uv, float t) {
  float scale = max(uSize.x / uMediaSize.x, uSize.y / uMediaSize.y);
  vec2 q = (uv - 0.5) * uSize / (uMediaSize * scale) + 0.5;
  vec4 c = texture(uMedia, vec2(q.x, 1.0 - q.y));
  return dot(c.rgb, vec3(0.2126, 0.7152, 0.0722));
}`,m=`#version 300 es
precision highp float;
uniform sampler2D uField;
uniform vec2 uView;
uniform vec2 uSize;
uniform vec4 uStops[8];
uniform int uCount;
uniform float uIntensity;
uniform vec2 uPointer;
uniform float uPointerOn;
out vec4 outColor;
${l}
void main() {
  vec2 uv = gl_FragCoord.xy / uView;
  float base = texture(uField, uv).r;
  float halo = textureLod(uField, uv, 1.0).r * 0.45
    + textureLod(uField, uv, 2.2).r * 0.35
    + textureLod(uField, uv, 3.4).r * 0.2;
  float v = base + 0.55 * halo;
  float over = max(v - 0.85, 0.0);
  v = min(v, 0.85) + over / (1.0 + 4.0 * over);
  v = clamp(v * uIntensity, 0.0, 1.0);
  // The pointer glow comes after intensity, so it shows on quiet fields too.
  vec2 away = (uv - uPointer) * uSize;
  v = min(v + uPointerOn * exp(-dot(away, away) / 28000.0) * (0.35 + 0.5 * v), 1.0);
  // Drop the faintest light, so empty areas stay empty instead of showing
  // a sparse, even grid.
  v = max(v - 0.02, 0.0) / 0.98;
  float on = step(bayer(gl_FragCoord.xy), v);
  float at = v * float(uCount - 1);
  int i = int(min(floor(at), float(uCount - 2)));
  outColor = mix(uStops[i], uStops[i + 1], at - float(i)) * on;
}`;function h(e,t,n=``){let r=`#version 300 es\n${n}${u}`;return{text:`${r}${e}\n${t}`,field:{source:e,offset:r.split(`
`).length-1}}}var g=.6,_=6,v=.75,y=10,b=6,x=8,S=[.35,.75,1],C=4096,w=2e3,T=37297,E=/^data:video\/|\.(mp4|webm|ogv|mov|m4v)(\?|#|$)/i,D=/^ERROR: \d+:(\d+): (.*)$/,O=[`uView`,`uSize`,`uAspect`,`uDot`,`uTime`,`uPointer`,`uPointerOn`,`uMediaSize`],k=[...O,`uGain`,`uColours`,`uPal`,`uMatrix`],A=[`uView`,`uSize`,`uStops`,`uCount`,`uIntensity`,`uPointer`,`uPointerOn`,`uMatrix`],j=null;function M(){let e=document.createElement(`canvas`);e.width=1,e.height=1;let t=e.getContext(`webgl2`,{alpha:!0,premultipliedAlpha:!0,antialias:!1,depth:!1,stencil:!1,powerPreference:`low-power`});if(!t)return null;let n=new Set,r=e=>{e.preventDefault(),j===a&&(j=null);for(let e of n)e(`lost`)},i=()=>{for(let e of n)e(`restored`)};e.addEventListener(`webglcontextlost`,r),e.addEventListener(`webglcontextrestored`,i);let a={canvas:e,gl:t,parallel:!!t.getExtension(`KHR_parallel_shader_compile`),maxDots:Math.min(C,t.getParameter(t.MAX_TEXTURE_SIZE)),programs:new Map,noise:new Map,refs:0,timer:0,members:new Set,owner:null,listeners:n,destroy(){j===a&&(j=null),e.removeEventListener(`webglcontextlost`,r),e.removeEventListener(`webglcontextrestored`,i),t.getExtension(`WEBGL_lose_context`)?.loseContext()}};return a}function ee(){return j??=M(),j?(window.clearTimeout(j.timer),j.refs+=1,j.owner?.detach(),j):null}function N(e,t){if(t&&e.members.delete(t),--e.refs,e.refs===1)for(let t of e.members)t.attach();e.refs>0||(e.timer=window.setTimeout(()=>{e.refs<=0&&e.destroy()},w))}function P(e,t,{text:n,field:r},i,a){let o=e.programs.get(t);if(o)return o;let{gl:s}=e,l={program:s.createProgram(),shaders:[],state:`pending`,uniforms:i,samplers:a,loc:{},field:r};if(e.programs.set(t,l),!l.program)return l.state=`failed`,l;for(let[e,t]of[[s.VERTEX_SHADER,c],[s.FRAGMENT_SHADER,n]]){let n=s.createShader(e);n?(s.shaderSource(n,t),s.compileShader(n),s.attachShader(l.program,n),l.shaders.push(n)):l.state=`failed`}return s.linkProgram(l.program),l}function F(e,t){let n=t?.source.split(`
`)??[],r=e.trim().split(`
`).map(e=>{let r=D.exec(e.trim()),i=r&&t?Number(r[1])-t.offset:0,a=n[i-1];return!r||a===void 0?e:`line ${i}: ${r[2]}\n    ${a.trim()}`});console.error(`Dither: the shader did not compile.\n${r.join(`
`)}`)}function I(e,t){let{gl:n,parallel:r}=e,{program:i}=t;if(t.state!==`pending`||!i)return t.state;if(r&&!n.getProgramParameter(i,T))return`pending`;let a=!!n.getProgramParameter(i,n.LINK_STATUS);a||F(t.shaders.map(e=>n.getShaderInfoLog(e)??``).join(`
`).trim()||(n.getProgramInfoLog(i)??``),t.field);for(let e of t.shaders)n.deleteShader(e);if(t.shaders=[],!a)return t.state=`failed`,`failed`;for(let e of t.uniforms)t.loc[e]=n.getUniformLocation(i,e);n.useProgram(i);for(let[e,r]of Object.entries(t.samplers))n.uniform1i(n.getUniformLocation(i,e),r);return t.state=`ready`,`ready`}function L(e,t){let n=t.map(t=>I(e,t));return n.includes(`failed`)?`failed`:n.includes(`pending`)?`pending`:`ok`}function R(e,t){let n=e.createTexture();e.bindTexture(e.TEXTURE_2D,n);let r=t?e.LINEAR_MIPMAP_LINEAR:e.LINEAR;return e.texParameteri(e.TEXTURE_2D,e.TEXTURE_MIN_FILTER,r),e.texParameteri(e.TEXTURE_2D,e.TEXTURE_MAG_FILTER,e.LINEAR),e.texParameteri(e.TEXTURE_2D,e.TEXTURE_WRAP_S,e.CLAMP_TO_EDGE),e.texParameteri(e.TEXTURE_2D,e.TEXTURE_WRAP_T,e.CLAMP_TO_EDGE),n}function z(e,t){let{gl:n}=e;n.activeTexture(n.TEXTURE2);let r=e.noise.get(t);if(r){n.bindTexture(n.TEXTURE_2D,r);return}let i=Uint8Array.from(atob(o[t]),e=>e.charCodeAt(0)),a=new Uint8Array(65536);for(let e=0;e<256;e+=1)for(let t=0;t<256;t+=1)a[e*256+t]=i[256+(i[t]+e)%256];let s=R(n,!1);n.pixelStorei(n.UNPACK_ALIGNMENT,1),n.texImage2D(n.TEXTURE_2D,0,n.R8,256,256,0,n.RED,n.UNSIGNED_BYTE,a),e.noise.set(t,s)}function B(e,t,n){let{canvas:r,maxDots:i}=e;r.width<t&&(r.width=Math.min(Math.ceil(t/256)*256,i)),r.height<n&&(r.height=Math.min(Math.ceil(n/256)*256,i))}function V(e,{ctx:t},n,r){t&&(t.globalCompositeOperation=`copy`,t.drawImage(e.canvas,0,e.canvas.height-r,n,r,0,0,n,r))}function H(e,t,n,r,i,{time:a,pointer:o}){e.uniform2f(t.uView,n[0],n[1]),e.uniform2f(t.uSize,r[0],r[1]),e.uniform1f(t.uAspect,r[0]/r[1]),e.uniform1f(t.uDot,i),e.uniform1f(t.uTime,a),e.uniform2f(t.uPointer,o[0],o[1]),e.uniform1f(t.uPointerOn,o[2])}function U(e,t,n){let{gl:r}=e,i=P(e,n.key,h(n.field,d),O,{uMedia:1,uNoise:2}),a=P(e,`dither`,{text:m},A,{uField:0}),o=R(r,!0),s=R(r,!1),c=r.createFramebuffer(),l=new Float32Array(32),u=null,f=!1,p=[1,1],_=[1,1],v=[1,1];return{resize(e,t,n,i){p=[e,t],v=[n,i],_=[Math.max(1,Math.round(n*g)),Math.max(1,Math.round(i*g))],r.activeTexture(r.TEXTURE0),r.bindTexture(r.TEXTURE_2D,o),r.texImage2D(r.TEXTURE_2D,0,r.RGBA,_[0],_[1],0,r.RGBA,r.UNSIGNED_BYTE,null),r.bindFramebuffer(r.FRAMEBUFFER,c),r.framebufferTexture2D(r.FRAMEBUFFER,r.COLOR_ATTACHMENT0,r.TEXTURE_2D,o,0),r.bindFramebuffer(r.FRAMEBUFFER,null)},setMedia(e){u=e,f=!0},draw(n){let d=L(e,[i,a]);if(d!==`ok`)return d;B(e,p[0],p[1]),r.useProgram(i.program);let m=u;if(m){let e=m instanceof HTMLVideoElement,t=e?m.videoWidth:m.naturalWidth,n=e?m.videoHeight:m.naturalHeight;if(!t||!n)return`pending`;r.activeTexture(r.TEXTURE1),r.bindTexture(r.TEXTURE_2D,s),(f||e)&&(r.texImage2D(r.TEXTURE_2D,0,r.RGBA,r.RGBA,r.UNSIGNED_BYTE,m),f=!1),r.uniform2f(i.loc.uMediaSize,t,n)}z(e,11),H(r,i.loc,_,v,v[0]/p[0],n),r.bindFramebuffer(r.FRAMEBUFFER,c),r.viewport(0,0,_[0],_[1]),r.drawArrays(r.TRIANGLES,0,3),r.bindFramebuffer(r.FRAMEBUFFER,null),r.activeTexture(r.TEXTURE0),r.bindTexture(r.TEXTURE_2D,o),r.generateMipmap(r.TEXTURE_2D);for(let[e,t]of n.stops.entries())l.set(t,e*4);return r.useProgram(a.program),r.uniform2f(a.loc.uView,p[0],p[1]),r.uniform2f(a.loc.uSize,v[0],v[1]),r.uniform4fv(a.loc.uStops,l),r.uniform1i(a.loc.uCount,n.stops.length),r.uniform1f(a.loc.uIntensity,n.intensity),r.uniform1i(a.loc.uMatrix,n.matrix),r.uniform2f(a.loc.uPointer,n.pointer[0],n.pointer[1]),r.uniform1f(a.loc.uPointerOn,n.pointer[2]),r.viewport(0,0,p[0],p[1]),r.drawArrays(r.TRIANGLES,0,3),V(e,t,p[0],p[1]),`ok`},dispose(){r.deleteTexture(o),r.deleteTexture(s),r.deleteFramebuffer(c)}}}function te(e,t,n,r){let{gl:i}=e,a=r.palette.length===2?`#define ONE_BIT
`:``,o=P(e,n.key,h(n.field,f,a),k,{uNoise:2}),s=new Float32Array(24),c=[1,1],l=[1,1];return{resize(e,t,n,r){c=[e,t],l=[n,r]},draw(n){let a=L(e,[o]);if(a!==`ok`)return a;B(e,c[0],c[1]),i.useProgram(o.program),z(e,r.seed??11);for(let[e,t]of n.stops.entries())s.set(t,e*4);return H(i,o.loc,c,l,l[0]/c[0],n),i.uniform1f(o.loc.uGain,n.intensity/v),i.uniform1i(o.loc.uColours,n.stops.length),i.uniform4fv(o.loc.uPal,s),i.uniform1i(o.loc.uMatrix,n.matrix),i.viewport(0,0,c[0],c[1]),i.drawArrays(i.TRIANGLES,0,3),V(e,t,c[0],c[1]),`ok`}}}function W(e,t,n){if(t)return{key:`field:media`,field:p,start:0,palette:null,media:t};if(n)return{key:`field:shader:${n}`,field:n,start:0,palette:null,media:null};let r=s[e];return`palette`in r?{key:`palette:${e}`,field:r.field,start:b,palette:r,media:null}:{key:`field:${e}`,field:r.field,start:y,palette:null,media:null}}function G(e,t){let n=typeof e==`string`?[e]:e;return t?n?.length?n.length===1?[`transparent`,n[0]]:n:t.palette:n?.length?n:[`currentColor`]}var K=null;function q(e,t){K??=document.createElement(`canvas`).getContext(`2d`,{willReadFrequently:!0});let n=K,r=document.createElement(`span`);r.style.display=`none`,e.appendChild(r);let i=t.map(e=>{if(r.style.color=``,r.style.color=e,!n)return[.5,.5,.5,1];n.clearRect(0,0,1,1),n.fillStyle=`#808080`,n.fillStyle=getComputedStyle(r).color,n.fillRect(0,0,1,1);let[t,i,a,o]=n.getImageData(0,0,1,1).data;return[t/255,i/255,a/255,o/255]});return r.remove(),i}function J(e,t){let n=e.length-1;return Array.from({length:t},(r,i)=>{let a=i/Math.max(t-1,1)*n,o=Math.floor(a),s=e[o],c=e[Math.min(o+1,n)],l=a-o,u=e=>s[e]+(c[e]-s[e])*l;return[u(0),u(1),u(2),u(3)]})}function ne(e,t,n){let r=q(e,t);if(!n&&r.length===1){let[e,t,n,i]=r[0];r=S.map(r=>[e,t,n,i*r])}let i=r.map(([e,t,n,r])=>[e*r,t*r,n*r,r]);return J(i,n?n.palette.length:Math.min(i.length,x))}function re(e,t,n){if(E.test(e)){let r=document.createElement(`video`);return r.crossOrigin=`anonymous`,r.muted=!0,r.loop=!0,r.playsInline=!0,r.preload=`auto`,r.addEventListener(`loadeddata`,()=>t(r),{once:!0}),r.addEventListener(`error`,n,{once:!0}),r.src=e,r}let r=new Image;return r.crossOrigin=`anonymous`,r.decoding=`async`,r.addEventListener(`load`,()=>t(r),{once:!0}),r.addEventListener(`error`,n,{once:!0}),r.src=e,r}var ie=`absolute top-0 left-0 opacity-0 transition-opacity duration-700 [image-rendering:pixelated] data-[ready]:opacity-100 motion-reduce:transition-none`,ae=e=>({canvas:e.canvas,ctx:null});function Y(){let e=document.createElement(`canvas`),t=e.getContext(`2d`);return t?{canvas:e,ctx:t}:null}var X=(e,t,n,r)=>e+(t-e)*(1-Math.exp(-n*r));function Z({pattern:e=`horizon`,src:t,shader:r,colors:o,matrix:s,dotSize:c=1,intensity:l=.75,speed:u=1,fps:d=30,interactive:f=!1,className:p,style:m,ref:h,...g}){let y=(0,i.useRef)(null),[b,x]=(0,i.useState)(!1),[S,C]=(0,i.useState)(`pending`),[w,T]=(0,i.useState)(0),E=(0,i.useMemo)(()=>W(e,t,r),[e,t,r]),D=G(o,E.palette),O=D.join(`|`),k=s??E.palette?.matrix??8,A=(0,i.useRef)({stops:D,matrix:k,intensity:l,speed:u,fps:d,interactive:f});(0,i.useEffect)(()=>{A.current={stops:D,matrix:k,intensity:l,speed:u,fps:d,interactive:f}});let j=(0,i.useRef)(null),M=(0,i.useRef)(0),P=(0,i.useRef)(!1),F=(0,i.useCallback)(e=>{y.current=e,typeof h==`function`?h(e):h&&(h.current=e)},[h]);(0,i.useEffect)(()=>{let e=y.current;if(!e)return;let t=new IntersectionObserver(([e])=>x(e.isIntersecting),{rootMargin:`100px`});return t.observe(e),()=>t.disconnect()},[]),(0,i.useEffect)(()=>{let e=y.current;if(!b||!e)return;let t=ee(),n=t&&(t.refs===1?ae(t):Y());if(!t||!n){t&&N(t,null),C(`dots`);return}n.canvas.className=ie;let r=E.palette?te(t,n,E,E.palette):U(t,n,E);C(P.current?`ready`:`pending`),P.current?n.canvas.dataset.ready=``:n.canvas.removeAttribute(`data-ready`),e.appendChild(n.canvas);let i=window.matchMedia(`(prefers-reduced-motion: reduce)`),a={x:0,y:0,clientX:0,clientY:0,on:!1,hover:0},o={stops:[],intensity:A.current.intensity,matrix:8,time:0,pointer:[0,0,0]},s=0,l=0,u=0,d=0,f=0,p=!1,m=!E.media,h=!1,g=!1,v=!1,x=()=>ne(e,A.current.stops,E.palette),S=()=>{if(!m||!f||h)return;o.intensity=A.current.intensity,o.matrix=A.current.matrix,o.time=E.start+M.current,o.pointer=[a.x/d,a.y/f,a.hover];let e=r.draw(o);e===`ok`?(n.canvas.dataset.ready=``,v||C(`ready`),P.current=!0,v=!0):e===`failed`?(h=!0,I(),C(`blank`)):u||=requestAnimationFrame(()=>{u=0,v||S()})},w=()=>{h=!0,n.canvas.removeAttribute(`data-ready`),I(),C(`dots`)},D=()=>{o.stops=x(),S()};j.current=D;let O=0,k=()=>{cancelAnimationFrame(O),O=requestAnimationFrame(D)},F=()=>{let i=e.clientWidth,a=e.clientHeight;if(!i||!a)return;let o=window.devicePixelRatio||1,s=Math.max(1,Math.round(c*o));for(;Math.ceil(i*o/s)>t.maxDots||Math.ceil(a*o/s)>t.maxDots;)s+=1;let l=s/o,u=Math.ceil(i*o/s),p=Math.ceil(a*o/s);d=u*l,f=p*l;let{canvas:m}=n;m.width=u,m.height=p,m.style.width=`${d}px`,m.style.height=`${f}px`,r.resize(u,p,d,f),S()},I=()=>{let e=m&&!h&&!document.hidden&&!i.matches;e&&!l?(s=0,l=requestAnimationFrame(L),z?.play().catch(()=>{})):!e&&l&&(cancelAnimationFrame(l),l=0,z?.pause())},L=t=>{l=requestAnimationFrame(L);let{speed:n,fps:r,interactive:i}=A.current;if(s&&t-s<1e3/Math.max(r,1)-2)return;let o=s?Math.min((t-s)/1e3,2):0;s=t,M.current+=o*n;let c=!1;if(i&&a.on){let t=e.getBoundingClientRect(),n=t.left+e.clientLeft,r=t.top+e.clientTop;c=a.clientX>=t.left&&a.clientX<=t.right&&a.clientY>=t.top&&a.clientY<=t.bottom;let i=a.clientX-n,s=r+f-a.clientY,l=c||a.hover>.002;a.x=l?X(a.x,i,_,o):i,a.y=l?X(a.y,s,_,o):s}let u=a.hover>.002;a.hover=X(a.hover,+!!c,4,o),!c&&!u&&(a.hover=0);let d=!!z||!E.media&&n!==0,m=c||u||p;p=c||u,(d||m)&&S()},R=E.media?re(E.media,e=>{g||(r.setMedia?.(e),m=!0,S(),I())},()=>{g||w()}):null,z=R instanceof HTMLVideoElement?R:null,B=t=>{if(t.pointerType!==`touch`||t.buttons!==0){if(!a.on){let n=e.getBoundingClientRect();a.x=t.clientX-n.left-e.clientLeft,a.y=n.top+e.clientTop+f-t.clientY}a.clientX=t.clientX,a.clientY=t.clientY,a.on=!0}},V=()=>{a.on=!1},H=e=>{e.pointerType===`touch`&&(a.on=!1)},W=null,G=()=>{W?.removeEventListener(`change`,K),W=window.matchMedia(`(resolution: ${window.devicePixelRatio}dppx)`),W.addEventListener(`change`,K)},K=()=>{F(),G()},q=e=>{e===`restored`?T(e=>e+1):w()};t.listeners.add(q);let J={detach:()=>{t.owner=null;let e=Y();if(!e){w();return}let{canvas:r}=n;e.canvas.className=r.className,e.canvas.style.cssText=r.style.cssText,e.canvas.width=r.width,e.canvas.height=r.height,r.dataset.ready!==void 0&&(e.canvas.dataset.ready=``),r.replaceWith(e.canvas),n.canvas=e.canvas,n.ctx=e.ctx,S()},attach:()=>{if(h||!n.ctx)return;let e=n.canvas,{canvas:r}=t;r.className=e.className,e.dataset.ready===void 0?r.removeAttribute(`data-ready`):r.dataset.ready=``,e.replaceWith(r),n.canvas=r,n.ctx=null,t.owner=J,F()}};t.members.add(J),n.ctx||(t.owner=J);let Z=new ResizeObserver(F),Q=new MutationObserver(k),$=window.matchMedia(`(prefers-color-scheme: dark)`);return o.stops=x(),F(),I(),G(),Z.observe(e),Q.observe(document.documentElement,{attributes:!0,attributeFilter:[`class`,`data-theme`]}),Q.observe(e,{attributes:!0,attributeFilter:[`class`,`style`]}),$.addEventListener(`change`,k),i.addEventListener(`change`,I),document.addEventListener(`visibilitychange`,I),window.addEventListener(`pointerdown`,B,{passive:!0}),window.addEventListener(`pointermove`,B,{passive:!0}),window.addEventListener(`pointerup`,H,{passive:!0}),window.addEventListener(`pointercancel`,H,{passive:!0}),document.documentElement.addEventListener(`pointerleave`,V),()=>{g=!0,cancelAnimationFrame(l),cancelAnimationFrame(u),cancelAnimationFrame(O),j.current=null,Z.disconnect(),Q.disconnect(),W?.removeEventListener(`change`,K),$.removeEventListener(`change`,k),i.removeEventListener(`change`,I),document.removeEventListener(`visibilitychange`,I),window.removeEventListener(`pointerdown`,B),window.removeEventListener(`pointermove`,B),window.removeEventListener(`pointerup`,H),window.removeEventListener(`pointercancel`,H),document.documentElement.removeEventListener(`pointerleave`,V),t.listeners.delete(q),z?(z.pause(),z.removeAttribute(`src`),z.load()):R?.removeAttribute(`src`),r.dispose?.(),t.owner===J&&(t.owner=null),N(t,J),n.canvas.remove()}},[b,E,c,w]),(0,i.useEffect)(()=>{j.current?.()},[O,l,k]);let I=E.palette?{backgroundColor:D[0],backgroundImage:`radial-gradient(${D.at(-1)} 0.6px, transparent 0.9px)`,backgroundSize:`${3*c}px ${3*c}px`,opacity:Math.min(1,l/v)}:{backgroundImage:`radial-gradient(${D.at(-1)} 0.6px, transparent 0.9px)`,backgroundSize:`${3*c}px ${3*c}px`,maskImage:`radial-gradient(ellipse at 50% 70%, black, transparent 75%)`,WebkitMaskImage:`radial-gradient(ellipse at 50% 70%, black, transparent 75%)`,opacity:Math.min(1,l)};return(0,a.jsx)(`div`,{ref:F,"data-slot":`dither`,"data-state":S===`dots`||S===`blank`?`fallback`:S,"aria-hidden":`true`,className:n(`pointer-events-none absolute inset-0 overflow-hidden rounded-[inherit]`,p),style:S===`dots`?{...m,...I}:m,...g})}export{Z as Dither};