<script lang="ts">
  import { onMount } from 'svelte';
  import type { Report } from '$lib/types';
  import type { FeatureCollection, Point } from 'geojson';

  import 'maplibre-gl/dist/maplibre-gl.css';

  let { reports } = $props<{ reports: Report[] }>();
  let container: HTMLDivElement;

  onMount(() => {
    let map: import('maplibre-gl').Map | null = null;

    void (async () => {
      const maplibregl = (await import('maplibre-gl')).default;

      map = new maplibregl.Map({
        container,
        style: 'https://demotiles.maplibre.org/style.json',
        center: [4.9041, 52.3676],
        zoom: 11
      });

      map.on('load', () => {
        const points: FeatureCollection<Point, { id: string; status: string; tags: string }> = {
          type: 'FeatureCollection',
          features: reports.map((report: Report) => ({
            type: 'Feature',
            geometry: {
              type: 'Point',
              coordinates: [report.location.lng, report.location.lat]
            },
            properties: {
              id: report.id,
              status: report.status,
              tags: report.tags.join(', ')
            }
          }))
        };

        map?.addSource('reports', {
          type: 'geojson',
          data: points,
          cluster: true,
          clusterRadius: 45
        });

        map?.addLayer({
          id: 'report-clusters',
          type: 'circle',
          source: 'reports',
          filter: ['has', 'point_count'],
          paint: {
            'circle-color': '#0b4a6f',
            'circle-radius': ['step', ['get', 'point_count'], 15, 10, 20, 30, 24],
            'circle-opacity': 0.75
          }
        });

        map?.addLayer({
          id: 'report-points',
          type: 'circle',
          source: 'reports',
          filter: ['!', ['has', 'point_count']],
          paint: {
            'circle-color': '#d9534f',
            'circle-radius': 7,
            'circle-stroke-width': 1,
            'circle-stroke-color': '#ffffff'
          }
        });
      });
    })();

    return () => map?.remove();
  });
</script>

<div class="map" bind:this={container}></div>

<style>
  .map {
    width: 100%;
    height: 60vh;
    min-height: 360px;
    border: 1px solid #dbe6ee;
    border-radius: 12px;
    overflow: hidden;
  }
</style>
