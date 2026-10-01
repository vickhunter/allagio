import {
    BoxGeometry, CatmullRomCurve3, CircleGeometry, DirectionalLight, Group,
    HemisphereLight, Mesh, MeshStandardMaterial,
    OrthographicCamera, Scene, Shape, ExtrudeGeometry, SphereGeometry,
    TubeGeometry, Vector3, WebGLRenderer,
} from 'three';

export function mountScene(stage, toggle, { paused = false, onPause = () => {} } = {}) {
    const canvas = document.createElement('canvas');
    const context = canvas.getContext('webgl2', { alpha: true, antialias: true, powerPreference: 'low-power' });
    if (!context) return () => {};
    const renderer = new WebGLRenderer({ canvas, context, alpha: true, antialias: true });
    renderer.setPixelRatio(Math.min(devicePixelRatio, 1.5));
    const scene = new Scene();
    const world = new Group();
    scene.add(world);
    scene.add(new HemisphereLight(0xfff4dd, 0x657d62, 2.6));
    const key = new DirectionalLight(0xffe5ba, 3.1);
    key.position.set(-3, 6, 7);
    scene.add(key);
    const fill = new DirectionalLight(0xcfe5cf, 1.4);
    fill.position.set(6, 3, -4);
    scene.add(fill);
    const camera = new OrthographicCamera();
    camera.position.set(5.5, 5, 9);
    camera.lookAt(0, 0, 0);
    const materials = new Map();
    const material = color => {
        if (!materials.has(color)) materials.set(color, new MeshStandardMaterial({ color, roughness: .62, metalness: .04 }));
        return materials.get(color);
    };
    const mesh = (geometry, color, parent = world) => {
        const object = new Mesh(geometry, material(color));
        parent.add(object);
        return object;
    };
    const rounded = (width, height, depth, radius = .12) => {
        const shape = new Shape();
        const x = -width / 2, y = -height / 2;
        shape.moveTo(x + radius, y);
        shape.lineTo(x + width - radius, y);
        shape.quadraticCurveTo(x + width, y, x + width, y + radius);
        shape.lineTo(x + width, y + height - radius);
        shape.quadraticCurveTo(x + width, y + height, x + width - radius, y + height);
        shape.lineTo(x + radius, y + height);
        shape.quadraticCurveTo(x, y + height, x, y + height - radius);
        shape.lineTo(x, y + radius);
        shape.quadraticCurveTo(x, y, x + radius, y);
        const geometry = new ExtrudeGeometry(shape, { depth, bevelEnabled: true, bevelSegments: 2, steps: 1, bevelSize: .03, bevelThickness: .03, curveSegments: 8 });
        geometry.translate(0, 0, -depth / 2);
        return geometry;
    };
    const stroke = (points, color, radius = .025, parent = world) => {
        const curve = new CatmullRomCurve3(points.map(point => new Vector3(...point)));
        const object = mesh(new TubeGeometry(curve, 32, radius, 6, false), color, parent);
        return { object, curve };
    };
    const base = mesh(new CircleGeometry(3.6, 64), 0x304c3d);
    base.rotation.x = -Math.PI / 2;
    base.position.y = -.72;
    base.scale.y = .84;
    for (let i = -4; i <= 4; i++) {
        const offset = i * .7;
        const length = Math.sqrt(3.45 ** 2 - offset ** 2);
        stroke([[-length, -.71, offset], [length, -.71, offset]], 0x45614b, .008);
        stroke([[offset, -.71, -length], [offset, -.71, length]], 0x45614b, .008);
    }
    function pedestal(x, z, size) {
        const disk = mesh(new CircleGeometry(size, 48), 0x21392c);
        disk.rotation.x = -Math.PI / 2;
        disk.position.set(x, -.69, z);
    }
    pedestal(-2, .15, 1.05);
    pedestal(0, .3, .9);
    pedestal(2.05, -.8, .85);
    pedestal(1.65, 1.5, .7);
    const product = new Group();
    product.position.set(-2, .14, .15);
    product.rotation.y = .12;
    world.add(product);
    mesh(rounded(1.55, 1.5, .24), 0xe5e6cd, product);
    const screen = mesh(rounded(1.27, 1.02, .025, .06), 0x263d31, product);
    screen.position.set(0, -.05, .15);
    stroke([[-.4, .12, .19], [-.23, -.02, .19], [-.4, -.16, .19]], 0xf4c182, .033, product);
    stroke([[-.05, -.16, .19], [.31, -.16, .19]], 0xc7d6b9, .028, product);
    for (let i = 0; i < 3; i++) {
        const dot = mesh(new SphereGeometry(.035, 12, 8), [0xd88c61, 0xd5b977, 0x95ac83][i], product);
        dot.position.set(-.53 + i * .13, .58, .15);
    }
    const hub = new Group();
    hub.position.set(0, -.06, .35);
    world.add(hub);
    mesh(rounded(1.05, 1.05, .75, .16), 0xdf7d52, hub);
    stroke([[-.23, -.22, .43], [.24, .24, .43]], 0xfff0ce, .047, hub);
    stroke([[-.08, .24, .43], [.24, .24, .43], [.24, -.07, .43]], 0xfff0ce, .047, hub);
    function article(x, y, z, scale, color) {
        const group = new Group();
        group.position.set(x, y, z);
        group.scale.setScalar(scale);
        group.rotation.y = -.16;
        world.add(group);
        mesh(rounded(1.2, 1.6, .17, .07), 0xf3efd8, group);
        const picture = mesh(rounded(.91, .45, .025, .025), color, group);
        picture.position.set(0, .37, .12);
        for (let i = 0; i < 3; i++) {
            const line = mesh(new BoxGeometry(.85 - i * .1, .035, .02), 0xa1b092, group);
            line.position.set(-i * .05, -.05 - i * .15, .12);
        }
        const link = mesh(rounded(.62, .16, .025, .025), 0xd79e69, group);
        link.position.set(-.13, -.58, .12);
        return group;
    }
    const page = article(2.05, .1, -.8, .93, 0x809c78);
    const second = article(1.65, -.16, 1.5, .65, 0xacbe93);
    const paths = [
        stroke([[-2, -.63, .4], [-1.5, -.63, 1], [-.6, -.63, 1], [0, -.63, .5]], 0x99af87, .02).curve,
        stroke([[0, -.63, .5], [.65, -.63, -.4], [1.4, -.63, -.7], [2.05, -.63, -.8]], 0x99af87, .02).curve,
        stroke([[0, -.63, .5], [.45, -.63, 1.4], [1, -.63, 1.6], [1.65, -.63, 1.5]], 0x99af87, .02).curve,
    ];
    const pulses = paths.map(() => mesh(new SphereGeometry(.05, 12, 8), 0xf5c681));
    let disposed = false;
    let frame = 0;
    let previous = 0;
    let elapsed = 0;
    let pointerX = 0;
    let pointerY = 0;
    function render(time) {
        if (disposed || paused || document.hidden || renderer.getContext().isContextLost()) return;
        frame = requestAnimationFrame(render);
        if (time - previous < 1000 / 30) return;
        elapsed += Math.min((time - previous) / 1000, .05);
        previous = time;
        world.rotation.y += (pointerX * .1 - .1 - world.rotation.y) * .06;
        world.rotation.x += (pointerY * .035 - world.rotation.x) * .06;
        hub.position.y = -.06 + Math.sin(elapsed * .75) * .035;
        page.position.y = .1 + Math.sin(elapsed * .8 + 1) * .02;
        second.position.y = -.16 + Math.sin(elapsed * .8 + 2) * .02;
        pulses.forEach((pulse, index) => pulse.position.copy(paths[index].getPointAt((elapsed * .11 + index * .3) % 1)));
        renderer.render(scene, camera);
    }
    const resume = () => {
        cancelAnimationFrame(frame);
        previous = performance.now();
        frame = requestAnimationFrame(render);
    };
    const resize = () => {
        const width = stage.clientWidth, height = stage.clientHeight;
        if (!width || !height || disposed) return;
        const halfHeight = 2.8;
        camera.left = -halfHeight * width / height;
        camera.right = halfHeight * width / height;
        camera.top = halfHeight;
        camera.bottom = -halfHeight;
        camera.near = .1;
        camera.far = 50;
        camera.updateProjectionMatrix();
        renderer.setSize(width, height, false);
        renderer.render(scene, camera);
    };
    const pointer = event => {
        const rect = stage.getBoundingClientRect();
        pointerX = (event.clientX - rect.left) / rect.width - .5;
        pointerY = (event.clientY - rect.top) / rect.height - .5;
    };
    const resetPointer = () => { pointerX = 0; pointerY = 0; };
    const pause = () => {
        paused = !paused;
        onPause(paused);
        toggle.textContent = paused ? 'Play illustration' : 'Pause illustration';
        if (paused) cancelAnimationFrame(frame);
        else resume();
    };
    const visibility = () => { if (document.hidden) cancelAnimationFrame(frame); else resume(); };
    const lost = event => { event.preventDefault(); cancelAnimationFrame(frame); toggle.hidden = true; stage.classList.remove('scene-ready'); };
    const restored = () => { resize(); toggle.hidden = false; stage.classList.add('scene-ready'); resume(); };
    const observer = new ResizeObserver(resize);
    stage.append(canvas);
    observer.observe(stage);
    stage.addEventListener('pointermove', pointer);
    stage.addEventListener('pointerleave', resetPointer);
    toggle.addEventListener('click', pause);
    document.addEventListener('visibilitychange', visibility);
    canvas.addEventListener('webglcontextlost', lost);
    canvas.addEventListener('webglcontextrestored', restored);
    resize();
    toggle.hidden = false;
    toggle.textContent = paused ? 'Play illustration' : 'Pause illustration';
    stage.classList.add('scene-ready');
    resume();
    return () => {
        if (disposed) return;
        disposed = true;
        cancelAnimationFrame(frame);
        observer.disconnect();
        stage.removeEventListener('pointermove', pointer);
        stage.removeEventListener('pointerleave', resetPointer);
        toggle.removeEventListener('click', pause);
        document.removeEventListener('visibilitychange', visibility);
        canvas.removeEventListener('webglcontextlost', lost);
        canvas.removeEventListener('webglcontextrestored', restored);
        world.traverse(object => { if (object.isMesh) object.geometry.dispose(); });
        materials.forEach(value => value.dispose());
        renderer.dispose();
        renderer.forceContextLoss();
        canvas.remove();
        toggle.hidden = true;
        stage.classList.remove('scene-ready');
    };
}
