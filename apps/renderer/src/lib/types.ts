export type S3ObjectEntry = {
  key: string;
  etag: string;
  lastModified: string | null;
  size?: number;
};

export type CategoryIndex = {
  name: string;
  slug: string;
  notes: {
    title: string;
    slug: string;
    isDraft?: boolean;
    isHome?: boolean;
  }[];
};

export type FlatNoteIndex = {
  key: string; // full S3 key
  relativeKey: string;
  title: string;
  slug: string;
  category: string | null;
  categorySlug?: string | null;
  tags: string[];
  html: string;
  preview: string;
  created: string | null;
  updated: string | null;
  breadcrumbs: { title: string; href: string | null }[];
  isHome?: boolean;
  etag?: string;
  isDraft?: boolean;
};

export type IndexResponse = {
  categories: CategoryIndex[];
  flat: FlatNoteIndex[];
};

export type NoteResponse = {
  slug: string;
  title: string;
  category: string | null;
  html: string;
  preview: string;
  tags: string[];
  created: string | null;
  updated: string | null;
  breadcrumbs?: { title: string; href: string | null }[];
  isDraft?: boolean;
};

export type IndexData = IndexResponse;
