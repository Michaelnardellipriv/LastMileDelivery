FROM postgres:16


COPY schema_LastMileDb.sql /scripts/schema_spedizioni.sql

EXPOSE 5432

HEALTHCHECK --interval=5s --timeout=3s --retries=10 \
  CMD pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" || exit 1
