create table buckets (
    id integer primary key autoincrement,
    created_at datetime,
    updated_at datetime,
    name text not null
);

create unique index idx_buckets_name on buckets(name);

create table object_contents (
    id integer primary key autoincrement,
    created_at datetime,
    updated_at datetime,
    bucket_id integer not null,
    sha256 text not null,
    size integer not null,
    path text not null,
    ref_count integer not null default 0,
    constraint fk_buckets_object_contents foreign key (bucket_id) references buckets(id) on update cascade on delete restrict
);

create unique index idx_object_contents_sha256_bucket on object_contents(sha256, bucket_id);
create index idx_object_contents_ref_count on object_contents(ref_count);

create table objects (
    id integer primary key autoincrement,
    created_at datetime,
    updated_at datetime,
    key text not null,
    bucket_id integer not null,
    content_id integer not null,
    constraint fk_object_contents_objects foreign key (content_id) references object_contents(id) on update cascade on delete restrict,
    constraint fk_buckets_objects foreign key (bucket_id) references buckets(id) on update cascade on delete cascade
);

create index idx_objects_content_id on objects(content_id);
create unique index idx_bucket_key on objects(key, bucket_id);
