CREATE TABLE public.users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255)
);

CREATE TABLE public.profiles (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES public.users(id),
    avatar_url TEXT,
    bio TEXT
);
