# Frontend Implementation Guide

Hướng dẫn chi tiết về cách implement frontend application tích hợp với FolkForm Auth Backend API.

## 📋 Mục Lục

- [API Client Setup](#api-client-setup)
- [Authentication Implementation](#authentication-implementation)
- [CRUD Operations](#crud-operations)
- [Error Handling](#error-handling)
- [State Management](#state-management)
- [Best Practices](#best-practices)

---

## API Client Setup

### 1. Tạo API Client Class

File: `src/services/apiClient.ts`

```typescript
const API_BASE_URL = process.env.REACT_APP_API_URL || 'http://localhost:8080/api/v1';

class ApiClient {
  private token: string | null = null;
  private hwid: string;

  constructor() {
    // Tạo hoặc lấy HWID từ localStorage
    this.hwid = this.getOrCreateHWID();
  }

  private getOrCreateHWID(): string {
    let hwid = localStorage.getItem('hwid');
    if (!hwid) {
      // Tạo HWID duy nhất
      hwid = this.generateHWID();
      localStorage.setItem('hwid', hwid);
    }
    return hwid;
  }

  private generateHWID(): string {
    // Sử dụng device fingerprint hoặc thư viện device-uuid
    // Ví dụ đơn giản:
    return `hwid_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
  }

  setToken(token: string) {
    this.token = token;
    localStorage.setItem('auth_token', token);
  }

  getToken(): string | null {
    return this.token || localStorage.getItem('auth_token');
  }

  getHWID(): string {
    return this.hwid;
  }

  clearToken() {
    this.token = null;
    localStorage.removeItem('auth_token');
  }

  async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<ApiResponse<T>> {
    const token = this.getToken();
    const headers: HeadersInit = {
      'Content-Type': 'application/json',
      ...options.headers,
    };

    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    const response = await fetch(`${API_BASE_URL}${endpoint}`, {
      ...options,
      headers,
    });

    const data = await response.json();

    if (!response.ok || data.status === 'error') {
      throw new ApiError(data.message, data.code, response.status);
    }

    return data;
  }

  // CRUD Methods
  async find<T>(collection: string, filter?: any, options?: any): Promise<T[]> {
    const params = new URLSearchParams();
    if (filter) params.append('filter', JSON.stringify(filter));
    if (options) params.append('options', JSON.stringify(options));
    
    const response = await this.request<{ data: T[] }>(
      `/${collection}/find?${params.toString()}`
    );
    return response.data;
  }

  async findOne<T>(collection: string, filter?: any): Promise<T> {
    const params = new URLSearchParams();
    if (filter) params.append('filter', JSON.stringify(filter));
    
    const response = await this.request<{ data: T }>(
      `/${collection}/find-one?${params.toString()}`
    );
    return response.data;
  }

  async findById<T>(collection: string, id: string): Promise<T> {
    const response = await this.request<{ data: T }>(
      `/${collection}/find-by-id/${id}`
    );
    return response.data;
  }

  async insertOne<T>(collection: string, data: any): Promise<T> {
    const response = await this.request<{ data: T }>(
      `/${collection}/insert-one`,
      {
        method: 'POST',
        body: JSON.stringify(data),
      }
    );
    return response.data;
  }

  async updateById<T>(
    collection: string,
    id: string,
    data: any
  ): Promise<T> {
    const response = await this.request<{ data: T }>(
      `/${collection}/update-by-id/${id}`,
      {
        method: 'PUT',
        body: JSON.stringify(data),
      }
    );
    return response.data;
  }

  async deleteById(collection: string, id: string): Promise<void> {
    await this.request(`/${collection}/delete-by-id/${id}`, {
      method: 'DELETE',
    });
  }

  async findWithPagination<T>(
    collection: string,
    page: number = 1,
    limit: number = 10,
    filter?: any
  ): Promise<PaginatedResponse<T>> {
    const params = new URLSearchParams({
      page: page.toString(),
      limit: limit.toString(),
    });
    if (filter) params.append('filter', JSON.stringify(filter));

    const response = await this.request<{ data: PaginatedResponse<T> }>(
      `/${collection}/find-with-pagination?${params.toString()}`
    );
    return response.data;
  }
}

// Types
interface ApiResponse<T> {
  code: number | string;
  message: string;
  data: T;
  status: 'success' | 'error';
}

interface PaginatedResponse<T> {
  page: number;
  limit: number;
  itemCount: number;
  items: T[];
}

class ApiError extends Error {
  constructor(
    message: string,
    public code: string,
    public statusCode: number
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

export const apiClient = new ApiClient();
export { ApiError };
```

---

## Authentication Implementation

### 1. Auth Service

File: `src/services/authService.ts`

```typescript
import { apiClient } from './apiClient';
import type { User } from '../types';

class AuthService {
  /**
   * Đăng nhập bằng Firebase ID Token
   * @param idToken Firebase ID Token từ Firebase Client SDK
   */
  async loginWithFirebase(idToken: string): Promise<User> {
    const hwid = apiClient.getHWID();
    const response = await apiClient.request<{ data: User }>(
      '/auth/login/firebase',
      {
        method: 'POST',
        body: JSON.stringify({
          idToken,
          hwid,
        }),
      }
    );

    if (response.data.token) {
      apiClient.setToken(response.data.token);
    }

    return response.data;
  }

  async logout(): Promise<void> {
    const hwid = apiClient.getHWID();
    await apiClient.request('/auth/logout', {
      method: 'POST',
      body: JSON.stringify({ hwid }),
    });
    apiClient.clearToken();
  }

  async getProfile(): Promise<User> {
    const response = await apiClient.request<{ data: User }>(
      '/auth/profile'
    );
    return response.data;
  }

  async updateProfile(name: string): Promise<User> {
    const response = await apiClient.request<{ data: User }>(
      '/auth/profile',
      {
        method: 'PUT',
        body: JSON.stringify({ name }),
      }
    );
    return response.data;
  }

  async getUserRoles() {
    const response = await apiClient.request<{ data: any[] }>(
      '/auth/roles'
    );
    return response.data;
  }
}

export const authService = new AuthService();
```

### 2. Firebase Integration

File: `src/services/firebaseService.ts`

```typescript
import { initializeApp } from 'firebase/app';
import { 
  getAuth, 
  signInWithEmailAndPassword,
  signInWithPopup,
  GoogleAuthProvider,
  FacebookAuthProvider,
  signInWithPhoneNumber,
  RecaptchaVerifier
} from 'firebase/auth';
import { authService } from './authService';

const firebaseConfig = {
  // Your Firebase config
};

const app = initializeApp(firebaseConfig);
const auth = getAuth(app);

class FirebaseService {
  /**
   * Đăng nhập bằng Email/Password
   */
  async loginWithEmail(email: string, password: string) {
    const userCredential = await signInWithEmailAndPassword(auth, email, password);
    const idToken = await userCredential.user.getIdToken();
    return await authService.loginWithFirebase(idToken);
  }

  /**
   * Đăng nhập bằng Google
   */
  async loginWithGoogle() {
    const provider = new GoogleAuthProvider();
    const userCredential = await signInWithPopup(auth, provider);
    const idToken = await userCredential.user.getIdToken();
    return await authService.loginWithFirebase(idToken);
  }

  /**
   * Đăng nhập bằng Facebook
   */
  async loginWithFacebook() {
    const provider = new FacebookAuthProvider();
    const userCredential = await signInWithPopup(auth, provider);
    const idToken = await userCredential.user.getIdToken();
    return await authService.loginWithFirebase(idToken);
  }

  /**
   * Đăng nhập bằng Phone OTP
   */
  async loginWithPhone(phoneNumber: string) {
    const recaptchaVerifier = new RecaptchaVerifier('recaptcha-container', {
      size: 'invisible',
    }, auth);
    
    const confirmationResult = await signInWithPhoneNumber(
      auth, 
      phoneNumber, 
      recaptchaVerifier
    );
    
    // User nhập OTP code
    const userCredential = await confirmationResult.confirm('123456'); // OTP code
    const idToken = await userCredential.user.getIdToken();
    return await authService.loginWithFirebase(idToken);
  }

  /**
   * Đăng xuất
   */
  async logout() {
    await auth.signOut();
    await authService.logout();
  }
}

export const firebaseService = new FirebaseService();
```

---

## CRUD Operations

### 1. User Service

File: `src/services/userService.ts`

```typescript
import { apiClient } from './apiClient';
import type { User, PaginatedResponse } from '../types';

class UserService {
  async findAll(filter?: any): Promise<User[]> {
    return apiClient.find<User>('user', filter);
  }

  async findOne(filter: any): Promise<User> {
    return apiClient.findOne<User>('user', filter);
  }

  async findById(id: string): Promise<User> {
    return apiClient.findById<User>('user', id);
  }

  async findWithPagination(
    page: number = 1,
    limit: number = 10,
    filter?: any
  ): Promise<PaginatedResponse<User>> {
    return apiClient.findWithPagination<User>('user', page, limit, filter);
  }

  async create(userData: Partial<User>): Promise<User> {
    return apiClient.insertOne<User>('user', userData);
  }

  async update(id: string, userData: Partial<User>): Promise<User> {
    return apiClient.updateById<User>('user', id, userData);
  }

  async delete(id: string): Promise<void> {
    return apiClient.deleteById('user', id);
  }
}

export const userService = new UserService();
```

### 2. Generic CRUD Service Pattern

File: `src/services/baseService.ts`

```typescript
import { apiClient } from './apiClient';
import type { PaginatedResponse } from '../types';

export class BaseService<T> {
  constructor(private collectionName: string) {}

  async findAll(filter?: any): Promise<T[]> {
    return apiClient.find<T>(this.collectionName, filter);
  }

  async findOne(filter: any): Promise<T> {
    return apiClient.findOne<T>(this.collectionName, filter);
  }

  async findById(id: string): Promise<T> {
    return apiClient.findById<T>(this.collectionName, id);
  }

  async findWithPagination(
    page: number = 1,
    limit: number = 10,
    filter?: any
  ): Promise<PaginatedResponse<T>> {
    return apiClient.findWithPagination<T>(
      this.collectionName, 
      page, 
      limit, 
      filter
    );
  }

  async create(data: Partial<T>): Promise<T> {
    return apiClient.insertOne<T>(this.collectionName, data);
  }

  async update(id: string, data: Partial<T>): Promise<T> {
    return apiClient.updateById<T>(this.collectionName, id, data);
  }

  async delete(id: string): Promise<void> {
    return apiClient.deleteById(this.collectionName, id);
  }
}
```

**Sử dụng:**

```typescript
import { BaseService } from './baseService';
import type { Role } from '../types';

export const roleService = new BaseService<Role>('role');
```

---

## Error Handling

### 1. Error Handler Utility

File: `src/utils/errorHandler.ts`

```typescript
import { ApiError } from '../services/apiClient';

export function handleApiError(error: unknown): string {
  if (error instanceof ApiError) {
    switch (error.code) {
      case 'AUTH_001':
        return 'Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại.';
      case 'AUTH_002':
        return 'Thông tin đăng nhập không chính xác.';
      case 'AUTH_003':
        return 'Bạn không có quyền thực hiện thao tác này.';
      case 'VAL_001':
        return 'Dữ liệu không hợp lệ. Vui lòng kiểm tra lại.';
      case 'DB_002':
        return 'Không tìm thấy dữ liệu.';
      default:
        return error.message || 'Đã xảy ra lỗi. Vui lòng thử lại.';
    }
  }

  if (error instanceof Error) {
    return error.message;
  }

  return 'Đã xảy ra lỗi không xác định.';
}
```

### 2. Error Interceptor (Axios example)

Nếu sử dụng Axios thay vì fetch:

```typescript
import axios from 'axios';
import { apiClient } from './apiClient';

const axiosInstance = axios.create({
  baseURL: 'http://localhost:8080/api/v1',
});

// Request interceptor - thêm token
axiosInstance.interceptors.request.use((config) => {
  const token = apiClient.getToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Response interceptor - xử lý lỗi
axiosInstance.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      // Redirect to login
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);
```

---

## State Management

### 1. React Context (Simple)

File: `src/contexts/AuthContext.tsx`

```typescript
import React, { createContext, useContext, useState, useEffect } from 'react';
import { authService } from '../services/authService';
import type { User } from '../types';

interface AuthContextType {
  user: User | null;
  loading: boolean;
  login: (idToken: string) => Promise<void>;
  logout: () => Promise<void>;
  updateProfile: (name: string) => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Check if user is logged in
    const token = localStorage.getItem('auth_token');
    if (token) {
      authService.getProfile()
        .then(setUser)
        .catch(() => {
          localStorage.removeItem('auth_token');
        })
        .finally(() => setLoading(false));
    } else {
      setLoading(false);
    }
  }, []);

  const login = async (idToken: string) => {
    const userData = await authService.loginWithFirebase(idToken);
    setUser(userData);
  };

  const logout = async () => {
    await authService.logout();
    setUser(null);
  };

  const updateProfile = async (name: string) => {
    const updatedUser = await authService.updateProfile(name);
    setUser(updatedUser);
  };

  return (
    <AuthContext.Provider value={{ user, loading, login, logout, updateProfile }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
```

### 2. Redux Toolkit (Advanced)

File: `src/store/slices/authSlice.ts`

```typescript
import { createSlice, createAsyncThunk } from '@reduxjs/toolkit';
import { authService } from '../../services/authService';
import type { User } from '../../types';

interface AuthState {
  user: User | null;
  loading: boolean;
  error: string | null;
}

const initialState: AuthState = {
  user: null,
  loading: false,
  error: null,
};

export const loginWithFirebase = createAsyncThunk(
  'auth/login',
  async (idToken: string) => {
    return await authService.loginWithFirebase(idToken);
  }
);

export const logout = createAsyncThunk(
  'auth/logout',
  async () => {
    await authService.logout();
  }
);

const authSlice = createSlice({
  name: 'auth',
  initialState,
  reducers: {},
  extraReducers: (builder) => {
    builder
      .addCase(loginWithFirebase.pending, (state) => {
        state.loading = true;
        state.error = null;
      })
      .addCase(loginWithFirebase.fulfilled, (state, action) => {
        state.loading = false;
        state.user = action.payload;
      })
      .addCase(loginWithFirebase.rejected, (state, action) => {
        state.loading = false;
        state.error = action.error.message || 'Login failed';
      })
      .addCase(logout.fulfilled, (state) => {
        state.user = null;
      });
  },
});

export default authSlice.reducer;
```

---

## Best Practices

### 1. Type Safety

- Luôn sử dụng TypeScript types từ `types-and-interfaces.md`
- Sử dụng type guards để kiểm tra types
- Tránh sử dụng `any`, sử dụng `unknown` thay thế

### 2. Error Handling

- Luôn wrap API calls trong try-catch
- Hiển thị user-friendly error messages
- Log errors để debug
- Xử lý 401 để redirect về login

### 3. Performance

- Sử dụng pagination cho danh sách lớn
- Cache data khi có thể
- Debounce search queries
- Lazy load components

### 4. Security

- Không lưu sensitive data trong localStorage
- Validate input trước khi gửi API
- Xử lý token expiration
- Sử dụng HTTPS trong production

### 5. Code Organization

```
src/
├── services/        # API services
├── types/          # TypeScript types
├── contexts/       # React contexts
├── hooks/         # Custom hooks
├── utils/         # Utilities
└── components/    # React components
```

---

**Cập nhật lần cuối**: 2025-12-10

