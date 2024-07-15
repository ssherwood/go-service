# Go Service

```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

create table address
(
    id          uuid primary key     default uuid_generate_v4(),
    version     int                  default 1,
    street      text        not null,
    city        text        not null,
    state_cd    varchar(2)  not null,
    postal_cd   text        not null,
    country_cd  varchar(2)  not null default 'US',
    longitude   float       not null default 39.509444,
    latitude    float       not null default -98.433056,
    modified_by text        not null default current_user,
    modified_at timestamptz not null default current_timestamp
) split into 5 tablets;

create table location
(
    id          uuid primary key     default uuid_generate_v4(),
    version     int                  default 1,
    name        text        not null,
    description text,
    address_id  uuid references address (id),
    metadata    jsonb       not null default '{}',
    active      bool        not null default true,
    modified_by text        not null default current_user,
    modified_at timestamptz not null default current_timestamp
) split into 5 tablets;
```

```sql
insert into address(id, street, city, state_cd, postal_cd)
select uuid('cdd7cacd-8e0a-4372-8ceb-' || lpad(seq::text, 12, '0')),
       seq || ' Main St',
       'New York',
       'NY',
       '10' || lpad(seq::text, 3, '0')
from generate_series(0, 1000) as seq;


insert into location(id, name, description, address_id)
select uuid('f9654e2a-dc0d-4423-8291-' || lpad(seq::text, 12, '0')),
       'Name ' || seq,
       'Description ' || seq,
       uuid('cdd7cacd-8e0a-4372-8ceb-' || lpad(((seq % 1000))::text, 12, '0'))
from generate_series(0, 5000) as seq;
```


## Run Jaeger

```shell
docker run --name jaeger \
  -e COLLECTOR_OTLP_ENABLED=true \
  -p 16686:16686 \
  -p 4317:4317 \
  -p 4318:4318 \
  jaegertracing/all-in-one:1.35
```
^ latest version (1.58) fails to accept connections, I fell back to this older version to get it working.  I'm not sure
the exact version that is breaking...