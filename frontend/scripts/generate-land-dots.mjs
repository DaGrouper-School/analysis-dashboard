// Builds the dot grid for the globe: points spread evenly over the sphere, kept where they
// fall on land. Run with `node scripts/generate-land-dots.mjs` after changing the spacing.
import { readFileSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { geoContains } from 'd3-geo';
import { feature } from 'topojson-client';

const require = createRequire(import.meta.url);
const world = JSON.parse(readFileSync(require.resolve('world-atlas/land-110m.json'), 'utf8'));
const land = feature(world, world.objects.land);

const STEP = 1.5; // degrees of latitude between rows
const dots = [];
for (let lat = -84; lat <= 84; lat += STEP) {
	// Fewer dots per row toward the poles, so spacing on the sphere stays even.
	const count = Math.max(1, Math.round((360 / STEP) * Math.cos((lat * Math.PI) / 180)));
	for (let i = 0; i < count; i++) {
		const lng = -180 + (i + 0.5) * (360 / count);
		if (geoContains(land, [lng, lat]))
			dots.push(Math.round(lat * 10) / 10, Math.round(lng * 10) / 10);
	}
}
writeFileSync(new URL('../src/lib/data/land-dots.json', import.meta.url), JSON.stringify(dots));
console.log(`${dots.length / 2} land dots`);
