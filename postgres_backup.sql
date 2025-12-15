--
-- PostgreSQL database dump
--

\restrict xIffqcftawIfn5cNxeV8M7uyh7D2VKBm4hbtfJI9JLsxs1QcWdcQqMRvm71m47Y

-- Dumped from database version 16.11
-- Dumped by pg_dump version 16.11

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: people; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.people (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(255) NOT NULL,
    role character varying(100) NOT NULL,
    birth character varying(50) NOT NULL,
    location character varying(255) NOT NULL,
    avatar text NOT NULL,
    bio text,
    children text[] DEFAULT '{}'::text[],
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.people OWNER TO postgres;

--
-- Name: permission_requests; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.permission_requests (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    user_email character varying(255) NOT NULL,
    requested_role character varying(20) NOT NULL,
    message text,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


ALTER TABLE public.permission_requests OWNER TO postgres;

--
-- Name: users; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.users (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    email character varying(255) NOT NULL,
    password_hash character varying(255) NOT NULL,
    is_admin boolean DEFAULT false,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    role character varying(20) DEFAULT 'viewer'::character varying
);


ALTER TABLE public.users OWNER TO postgres;

--
-- Data for Name: people; Type: TABLE DATA; Schema: public; Owner: postgres
--

INSERT INTO public.people VALUES ('dcfaeaac-5e3d-408e-8e39-86c84681b6b9', 'Mary Johnson', 'Nephew', '1971', 'Tokyo, Japan', 'https://api.dicebear.com/7.x/avataaars/svg?seed=Mary Johnson&backgroundColor=b6e3f4', 'Veterinarian who cares deeply about animals.', '{965f80cf-f5d8-4066-87f6-c40b541b73d9}', '2025-12-14 22:33:29.955134', '2025-12-15 11:36:15.018524');
INSERT INTO public.people VALUES ('5cc8452c-c005-4fd6-9daa-7dbb393a3d68', 'Mary Thompson', 'Nephew', '1940', 'New York, USA', 'https://api.dicebear.com/7.x/avataaars/svg?seed=Mary Thompson&backgroundColor=b6e3f4', 'Retired teacher who loves classical music.', '{9852cddc-85b8-4213-bcb8-b1aee980ba9b}', '2025-12-15 09:57:44.574401', '2025-12-15 11:36:43.444551');
INSERT INTO public.people VALUES ('07ecac22-ecc3-4b06-b339-78c2f15cfbc9', 'Mark Davis', 'Daughter', '1975', 'Sydney, Australia', 'https://api.dicebear.com/7.x/avataaars/svg?seed=Mark Davis&backgroundColor=b6e3f4', 'Architect with an eye for modern design.', '{}', '2025-12-15 13:01:47.395932', '2025-12-15 13:01:47.395932');
INSERT INTO public.people VALUES ('9852cddc-85b8-4213-bcb8-b1aee980ba9b', 'Susan Johnson', 'Sister', '1943', 'Tokyo, Japan', 'https://api.dicebear.com/7.x/avataaars/svg?seed=Susan Johnson&backgroundColor=b6e3f4', 'Artist with a passion for painting landscapes.', '{07ecac22-ecc3-4b06-b339-78c2f15cfbc9}', '2025-12-15 11:36:43.444551', '2025-12-15 13:01:47.395932');
INSERT INTO public.people VALUES ('965f80cf-f5d8-4066-87f6-c40b541b73d9', 'Linda Thompson', 'Nephew', '1948', 'Sheffield, UK', 'https://api.dicebear.com/7.x/avataaars/svg?seed=Linda Thompson&backgroundColor=b6e3f4', 'Artist with a passion for painting landscapes.', '{5cc8452c-c005-4fd6-9daa-7dbb393a3d68}', '2025-12-15 09:57:34.62111', '2025-12-15 13:04:41.191857');


--
-- Data for Name: permission_requests; Type: TABLE DATA; Schema: public; Owner: postgres
--

INSERT INTO public.permission_requests VALUES ('956d416e-aeb6-4031-b548-2f89d5019169', '9add9952-553b-4c1e-ae0a-9b5b3b017f9b', 'amirimohammad689@gmail.com', 'editor', '', 'approved', '2025-12-14 22:25:55.134619', '2025-12-14 22:26:31.970988');


--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: postgres
--

INSERT INTO public.users VALUES ('b83accd1-7101-47e8-8351-9fd0d80c340a', 'mohammadamiri.py@gmail.com', '$2a$10$tFoNYSUd91.QX17DFAaeMe9K4s8KJlyDqvM4lb6izaYUTDpkuUsxu', true, '2025-12-13 15:28:25.554519', '2025-12-13 15:28:25.554519', 'admin');
INSERT INTO public.users VALUES ('9add9952-553b-4c1e-ae0a-9b5b3b017f9b', 'amirimohammad689@gmail.com', '$2a$10$anpn9YRCsTW3K70M2P0pe.Mwa8XDRMAlzX6Ul8cQRfP7lfKIavTvK', false, '2025-12-14 21:53:04.354896', '2025-12-14 22:26:31.970988', 'editor');


--
-- Name: people people_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.people
    ADD CONSTRAINT people_pkey PRIMARY KEY (id);


--
-- Name: permission_requests permission_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.permission_requests
    ADD CONSTRAINT permission_requests_pkey PRIMARY KEY (id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: idx_people_name; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_people_name ON public.people USING btree (name);


--
-- Name: idx_people_role; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_people_role ON public.people USING btree (role);


--
-- Name: idx_permission_requests_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_permission_requests_status ON public.permission_requests USING btree (status);


--
-- Name: idx_permission_requests_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_permission_requests_user_id ON public.permission_requests USING btree (user_id);


--
-- Name: permission_requests fk_user; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.permission_requests
    ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: permission_requests permission_requests_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.permission_requests
    ADD CONSTRAINT permission_requests_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict xIffqcftawIfn5cNxeV8M7uyh7D2VKBm4hbtfJI9JLsxs1QcWdcQqMRvm71m47Y

