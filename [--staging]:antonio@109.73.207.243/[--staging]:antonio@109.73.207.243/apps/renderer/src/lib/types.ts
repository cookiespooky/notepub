export type S3ObjectEntry = {
  key: string;
  etag: string;
  lastModified: string | null;
  size?: number;
};

export type IndexTreeNode = {
  title: string;
  path: string[];
  children: {
    title: string;
    slug: string;
  }[];
  folders?: IndexTreeNode[];
};

export type FlatNoteIndex = {
  key: string; // full S3 key
  relativeKey: string;
  title: string;
  slug: string;
  tags: string[];
  html: string;
  preview: string;
  created: string | null;
  updated: string | null;
  breadcrumbs: { title: string; href: string | null }[];
  isFolderIndex?: boolean;
  isHome?: boolean;
  etag?: string;
};

export type IndexResponse = {
  tree: IndexTreeNode[];
  flat: FlatNoteIndex[];
};

export type NoteResponse = {
  slug: string;
  title: string;
  html: string;
  tags: string[];
  created: string | null;
  updated: string | null;
  breadcrumbs?: { title: string; href: string | null }[];
};

export type FolderListing = {
  title: string;
  slugPath: string[];
  path: string[];
  breadcrumbs: { title: string; href: string | null }[];
  folders: { title: string; slugPath: string[] }[];
  notes: { title: string; slug: string }[];
};

export type FolderMeta = {
  path: string;
  title?: string;
  slug?: string;
  etag?: string;
  renderVersion?: string;
};

export type IndexData = IndexResponse & {
  folderMeta: Map<string, FolderMeta>;
};
