import axios from 'axios';

// In production (built by CI/CD), VITE_API_BASE_URL is set to "/api"
// so all requests become relative paths served through CloudFront.
// In development, the Vite proxy handles /api → localhost:8080.
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '/api';

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  withCredentials: true, // required for httpOnly cookie sessions
  headers: {
    'Content-Type': 'application/json',
  },
});

// Global response interceptor: redirect to /login on 401
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);
