import axios from "axios";

export const TOKEN_KEY = "volcamelli_token";
export const USER_KEY = "volcamelli_user";

// Todos los pedidos van al gateway. En desarrollo los pasa Vite y en Docker los pasa NGINX.
const httpClient = axios.create({
  baseURL: "/api",
  headers: {
    "Content-Type": "application/json",
  },
});

httpClient.interceptors.request.use((config) => {
  const token = localStorage.getItem(TOKEN_KEY);
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Si el token venció, el gateway responde 401: borramos la sesión y volvemos al login.
httpClient.interceptors.response.use(
  (response) => response,
  (error) => {
    const isLogin = error.config?.url?.includes("/auth/login");
    if (error.response?.status === 401 && !isLogin) {
      localStorage.removeItem(TOKEN_KEY);
      localStorage.removeItem(USER_KEY);
      window.location.href = "/login";
    }
    return Promise.reject(error);
  }
);

// errorMessage saca el mensaje que manda el sistema ({code, message}), que ya está pensado para el usuario.
export function errorMessage(err, fallback) {
  return err.response?.data?.message || fallback;
}

export default httpClient;
