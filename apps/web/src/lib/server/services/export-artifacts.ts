import { stringify } from 'csv-stringify/sync';
import { PDFDocument, StandardFonts, rgb } from 'pdf-lib';
import type { ExportArtifacts, Report } from '$lib/types';

interface ExportInput {
  reports: Report[];
  periodStart: string;
  periodEnd: string;
}

const buildCsv = (reports: Report[]): string => {
  const rows = reports.map((report) => ({
    report_id: report.id,
    public_id: report.publicId,
    created_at: report.createdAt,
    status: report.status,
    lat: report.location.lat,
    lng: report.location.lng,
    accuracy_m: report.location.accuracy_m,
    tags: report.tags.join('|'),
    note: report.note ?? '',
    dedupe_group_id: report.dedupeGroupId ?? ''
  }));

  return stringify(rows, {
    header: true,
    columns: [
      'report_id',
      'public_id',
      'created_at',
      'status',
      'lat',
      'lng',
      'accuracy_m',
      'tags',
      'note',
      'dedupe_group_id'
    ]
  });
};

const buildGeoJson = (reports: Report[]): string => {
  const featureCollection = {
    type: 'FeatureCollection',
    features: reports.map((report) => ({
      type: 'Feature',
      geometry: {
        type: 'Point',
        coordinates: [report.location.lng, report.location.lat]
      },
      properties: {
        report_id: report.id,
        public_id: report.publicId,
        created_at: report.createdAt,
        status: report.status,
        tags: report.tags,
        dedupe_group_id: report.dedupeGroupId
      }
    }))
  };

  return JSON.stringify(featureCollection, null, 2);
};

const getTagCounts = (reports: Report[]): Array<{ tag: string; count: number }> => {
  const counter = new Map<string, number>();

  for (const report of reports) {
    for (const tag of report.tags) {
      counter.set(tag, (counter.get(tag) ?? 0) + 1);
    }
  }

  return [...counter.entries()]
    .map(([tag, count]) => ({ tag, count }))
    .sort((a, b) => b.count - a.count);
};

const buildPdf = async ({ reports, periodStart, periodEnd }: ExportInput): Promise<Uint8Array> => {
  const pdf = await PDFDocument.create();
  const page = pdf.addPage([595, 842]);
  const font = await pdf.embedFont(StandardFonts.Helvetica);

  page.drawText('ZwerfFiets Municipal Summary', {
    x: 48,
    y: 790,
    size: 18,
    font,
    color: rgb(0.08, 0.08, 0.08)
  });

  page.drawText(`Period: ${periodStart} - ${periodEnd}`, {
    x: 48,
    y: 762,
    size: 11,
    font
  });

  page.drawText(`Total reports: ${reports.length}`, {
    x: 48,
    y: 742,
    size: 11,
    font
  });

  const statusCounts = reports.reduce<Record<string, number>>((acc, report) => {
    acc[report.status] = (acc[report.status] ?? 0) + 1;
    return acc;
  }, {});

  let cursorY = 720;
  page.drawText('Status distribution', { x: 48, y: cursorY, size: 11, font });
  cursorY -= 16;

  for (const [status, count] of Object.entries(statusCounts).sort((a, b) => b[1] - a[1])) {
    page.drawText(`- ${status}: ${count}`, { x: 60, y: cursorY, size: 10, font });
    cursorY -= 14;
  }

  cursorY -= 8;
  page.drawText('Top tags', { x: 48, y: cursorY, size: 11, font });
  cursorY -= 16;

  for (const tag of getTagCounts(reports).slice(0, 10)) {
    page.drawText(`- ${tag.tag}: ${tag.count}`, { x: 60, y: cursorY, size: 10, font });
    cursorY -= 14;
    if (cursorY < 80) {
      break;
    }
  }

  return await pdf.save();
};

export const buildExportArtifacts = async (
  reports: Report[],
  periodStart: string,
  periodEnd: string
): Promise<ExportArtifacts> => {
  const sortedReports = [...reports].sort((left, right) => {
    const byDate = left.createdAt.localeCompare(right.createdAt);
    if (byDate !== 0) {
      return byDate;
    }
    return left.id - right.id;
  });

  return {
    csv: buildCsv(sortedReports),
    geojson: buildGeoJson(sortedReports),
    pdf: await buildPdf({ reports: sortedReports, periodStart, periodEnd })
  };
};
