import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { FormEvent, useState } from "react";
import { Navigate, Route, Routes, useNavigate } from "react-router";
import { EnterBox } from "./motion";
import { fetchMe, login } from "./api";
import {
  AccountPage,
  CategoryCreatePage,
  CategoryPage,
  ItemCreatePage,
  ItemEditPage,
  ItemListPage,
  LocationCreatePage,
  LocationListPage,
  LocationPage,
  RequireAuth,
  ReturnListPage,
  TrashItemPage,
  TrashListPage,
} from "./pages";
import styles from "./styles.module.css";
import { ItemDetailPage } from './itemDetail';
import { CategoryDirectoryPage, LocationDirectoryPage } from './directory';

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<RequireAuth />}>
        <Route path="/" element={<ItemListPage />} />
        <Route path="/items/new" element={<ItemCreatePage />} />
        <Route path="/items/:id" element={<ItemDetailPage />} />
        <Route path="/items/:id/edit" element={<ItemEditPage />} />
        <Route path="/locations/new" element={<LocationCreatePage />} />
        <Route path="/locations" element={<LocationDirectoryPage />} />
        <Route path="/locations/manage" element={<LocationListPage />} />
        <Route path="/locations/:id" element={<LocationPage />} />
        <Route path="/categories/new" element={<CategoryCreatePage />} />
        <Route path="/categories" element={<CategoryDirectoryPage />} />
        <Route path="/categories/:id" element={<CategoryPage />} />
        <Route path="/returns" element={<ReturnListPage />} />
        <Route path="/account" element={<AccountPage />} />
        <Route path="/trash" element={<TrashListPage />} />
        <Route path="/trash/:id" element={<TrashItemPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

function LoginPage() {
  const me = useQuery({ queryKey: ["me"], queryFn: fetchMe, retry: false });
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const mutation = useMutation({
    mutationFn: () => login(username, password),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["me"] });
      navigate("/");
    },
    onError: (err: Error) => setError(err.message),
  });

  if (me.isLoading) {
    return <p className={styles.page}>正在确认登录状态</p>;
  }
  if (me.data) {
    return <Navigate to="/" replace />;
  }

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    setError("");
    mutation.mutate();
  }

  return (
    <main className={styles.loginPage}>
      <EnterBox className={styles.loginPanel}>
        <h1 className={styles.title}>有处</h1>
        <p className={styles.tagline}>家里的东西在哪儿</p>
        <form onSubmit={onSubmit}>
          <label className={styles.field}>
            用户名
            <input
              name="username"
              autoComplete="username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
            />
          </label>
          <label className={styles.field}>
            密码
            <input
              name="password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
          </label>
          <button className={styles.buttonPrimary} type="submit" disabled={mutation.isPending}>
            登录
          </button>
        </form>
        {error ? <p className={styles.error}>{error}</p> : null}
      </EnterBox>
    </main>
  );
}
