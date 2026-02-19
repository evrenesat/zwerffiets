(() => {
  const DEFAULT_CENTER = [5.2913, 52.1326]; // Center of Netherlands
  const DEFAULT_ZOOM = 7; // Show whole country by default
  const CLUSTER_RADIUS = 45;

  const OSM_RASTER_STYLE = {
    version: 8,
    sources: {
      osm: {
        type: 'raster',
        tiles: ['https://tile.openstreetmap.org/{z}/{x}/{y}.png'],
        tileSize: 256,
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
      }
    },
    layers: [{ id: 'osm-tiles', type: 'raster', source: 'osm' }]
  };

  const initMap = () => {
    const mapElement = document.getElementById('admin-map');
    if (!mapElement || typeof window.maplibregl === 'undefined') {
      return;
    }

    const rawData = Array.isArray(window.__ADMIN_MAP_DATA__) ? window.__ADMIN_MAP_DATA__ : [];
    const features = rawData
      .filter((entry) => typeof entry.lat === 'number' && typeof entry.lng === 'number')
      .map((entry) => ({
        type: 'Feature',
        geometry: {
          type: 'Point',
          coordinates: [entry.lng, entry.lat]
        },
        properties: {
          id: entry.id,
          publicId: entry.publicId,
          status: entry.statusLabel,
          tags: entry.tags,
          address: entry.address,
          city: entry.city
        }
      }));

    const map = new window.maplibregl.Map({
      container: mapElement,
      style: OSM_RASTER_STYLE,
      center: DEFAULT_CENTER,
      zoom: DEFAULT_ZOOM
    });

    if (features.length > 0) {
      const bounds = new window.maplibregl.LngLatBounds();
      features.forEach((feature) => {
        bounds.extend(feature.geometry.coordinates);
      });
      map.fitBounds(bounds, { padding: 50, maxZoom: 15 });
    }

    map.on('load', () => {
      map.addSource('reports', {
        type: 'geojson',
        data: {
          type: 'FeatureCollection',
          features
        },
        cluster: true,
        clusterRadius: CLUSTER_RADIUS
      });

      map.addLayer({
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

      map.addLayer({
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

      map.on('click', 'report-points', (e) => {
        const feature = e.features[0];
        const { id, publicId, status, tags, address, city } = feature.properties;
        const coordinates = feature.geometry.coordinates.slice();

        const html = `
          <div class="map-popup">
            <strong><a href="/bikeadmin/reports/${id}">${publicId}</a></strong><br>
            <span class="status">${status}</span><br>
            <span class="address">${address || '-'}</span>, <span class="city">${city || '-'}</span><br>
            <small class="tags">${tags}</small>
          </div>
        `;

        new window.maplibregl.Popup().setLngLat(coordinates).setHTML(html).addTo(map);
      });

      map.on('mouseenter', 'report-points', () => {
        map.getCanvas().style.cursor = 'pointer';
      });

      map.on('mouseleave', 'report-points', () => {
        map.getCanvas().style.cursor = '';
      });
    });
  };

  const initDetailMap = () => {
    const el = document.getElementById('detail-map');
    if (!el || typeof window.maplibregl === 'undefined') {
      return;
    }

    const lat = parseFloat(el.dataset.lat);
    const lng = parseFloat(el.dataset.lng);
    if (isNaN(lat) || isNaN(lng)) {
      return;
    }

    const DETAIL_MAP_ZOOM = 15;

    const map = new window.maplibregl.Map({
      container: el,
      style: OSM_RASTER_STYLE,
      center: [lng, lat],
      zoom: DETAIL_MAP_ZOOM,
      interactive: false
    });

    new window.maplibregl.Marker().setLngLat([lng, lat]).addTo(map);
  };

  const initPhotoModal = () => {
    const thumbs = Array.from(document.querySelectorAll('[data-photo-gallery] [data-photo-url]'));
    if (thumbs.length === 0) {
      return;
    }

    const modal = document.getElementById('photo-modal');
    const image = document.getElementById('photo-modal-image');
    if (!modal || !image) {
      return;
    }

    const closeElements = Array.from(modal.querySelectorAll('[data-photo-close]'));
    const nextButton = modal.querySelector('[data-photo-next]');
    const previousButton = modal.querySelector('[data-photo-prev]');
    let activeIndex = -1;

    const openAt = (index) => {
      activeIndex = index;
      const source = thumbs[activeIndex];
      image.setAttribute('src', source.getAttribute('data-photo-url') || '');
      image.setAttribute('alt', source.getAttribute('data-photo-alt') || '');
      modal.classList.remove('hidden');
    };

    const close = () => {
      activeIndex = -1;
      modal.classList.add('hidden');
      image.setAttribute('src', '');
    };

    const shift = (delta) => {
      if (activeIndex < 0 || thumbs.length === 0) {
        return;
      }
      activeIndex = (activeIndex + delta + thumbs.length) % thumbs.length;
      openAt(activeIndex);
    };

    thumbs.forEach((thumb, index) => {
      thumb.addEventListener('click', () => {
        openAt(index);
      });
    });

    closeElements.forEach((element) => {
      element.addEventListener('click', close);
    });

    if (nextButton) {
      nextButton.addEventListener('click', () => shift(1));
    }
    if (previousButton) {
      previousButton.addEventListener('click', () => shift(-1));
    }

    window.addEventListener('keydown', (event) => {
      if (modal.classList.contains('hidden')) {
        return;
      }
      if (event.key === 'Escape') {
        close();
      }
      if (event.key === 'ArrowRight') {
        shift(1);
      }
      if (event.key === 'ArrowLeft') {
        shift(-1);
      }
    });
  };

  const initBulkActions = () => {
    const selectAll = document.getElementById("select-all");
    const bulkBar = document.getElementById("bulk-bar");
    const bulkCount = document.getElementById("bulk-count");
    const bulkStatus = document.getElementById("bulk-status");
    const bulkSubmit = document.getElementById("bulk-submit");
    const checkboxes = () => Array.from(document.querySelectorAll(".row-checkbox"));

    if (!selectAll || !bulkBar) {
      return;
    }

    const updateState = () => {
      const checked = checkboxes().filter((cb) => cb.checked);
      const count = checked.length;
      bulkCount.textContent = count + " geselecteerd";
      bulkSubmit.disabled = count === 0 || bulkStatus.value === "";
      selectAll.checked = count > 0 && count === checkboxes().length;
      selectAll.indeterminate = count > 0 && count < checkboxes().length;
    };

    selectAll.addEventListener("change", () => {
      checkboxes().forEach((cb) => {
        cb.checked = selectAll.checked;
      });
      updateState();
    });

    document.addEventListener("change", (event) => {
      if (
        event.target.classList.contains("row-checkbox") ||
        event.target === bulkStatus
      ) {
        updateState();
      }
    });
  };

  initMap();
  initDetailMap();
  initPhotoModal();
  initBulkActions();
})();
