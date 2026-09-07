SELECT EXISTS (SELECT 1 FROM lab.posts WHERE id = :'post_id'::bigint);
