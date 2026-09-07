-- Last initialization step. A missing marker makes future starts fail closed.
COPY (SELECT 'ready') TO '/var/lib/postgresql/18/docker/.ddia-primary-ready';
